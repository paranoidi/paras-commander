package fileindex

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/fschange"
)

const (
	// tempMovePrefix names the throw-away record names a chained batch of renames passes through.
	tempMovePrefix = ".pc-fileindex-move-"
	// busyRetries and busyBackoff bound how long a record change waits for another writer (a scan
	// committing a batch): SQLite refuses a deferred transaction that became stale at once instead
	// of waiting for its busy timeout.
	busyRetries = 20
	busyBackoff = 50 * time.Millisecond
)

// Apply makes the index follow filesystem changes pc made on disk (fschange.Sink, ADR 0023): a
// removed path loses its record, a renamed one moves its record (a record at the target is
// dropped first, as the rename replaced the file). Paths the index does not know, Created and Dirty
// changes are ignored: new and changed content waits for a scan. Apply only queues the batch; one
// goroutine applies queued batches in order, and it does nothing while the index is not
// available (it never prompts or creates a database). OnChange fires once per pass that changed a
// record. A nil Index ignores everything.
func (x *Index) Apply(batch []fschange.Change) {
	if x == nil || len(batch) == 0 {
		return
	}
	x.applyMu.Lock()
	x.applyPending = append(x.applyPending, slices.Clone(batch))
	if x.applyRunning {
		x.applyMu.Unlock()
		return
	}
	x.applyRunning = true
	done := make(chan struct{})
	x.applyDone = done
	x.applyMu.Unlock()
	go x.drainApplies(done)
}

// drainApplies applies queued batches until none are left, then ends (the next Apply starts it
// again).
func (x *Index) drainApplies(done chan struct{}) {
	defer close(done)
	for {
		x.applyMu.Lock()
		batches := x.applyPending
		x.applyPending = nil
		if len(batches) == 0 {
			x.applyRunning = false
			x.applyMu.Unlock()
			return
		}
		x.applyMu.Unlock()
		d := x.openQuietly()
		if d == nil {
			continue // not available (or closed): nothing to follow
		}
		changed := false
		x.writeMu.Lock() // pc's own index writes (Backend.Rename, Remove) do not overlap this
		for _, b := range batches {
			if applyBatch(d, b) {
				changed = true
			}
		}
		x.writeMu.Unlock()
		if changed {
			x.changed()
		}
	}
}

// WaitApplied returns once the drain running now (if any) has ended, so every change queued before
// the call is applied (tests, Close).
func (x *Index) WaitApplied() {
	x.applyMu.Lock()
	done := x.applyDone
	x.applyMu.Unlock()
	if done != nil {
		<-done
	}
}

// openQuietly returns the open database, or nil when it is not available; unlike DB it never
// reports an open failure.
func (x *Index) openQuietly() *db.DB {
	x.mu.Lock()
	defer x.mu.Unlock()
	d, err := x.dbLocked()
	if err != nil {
		return nil
	}
	return d
}

// move is one record rename of a batch, with the paths as the index records them.
type move struct {
	from, to string
	dir      bool
}

// applyBatch applies one batch of changes and reports whether a record changed. Errors leave the
// records as they were: the next scan puts the index right.
func applyBatch(d *db.DB, batch []fschange.Change) (changed bool) {
	var moves []move
	for _, c := range batch {
		switch c.Kind {
		case fschange.Removed:
			if p, err := indexedPath(c.Path); err == nil && removeRecord(d, p) {
				changed = true
			}
		case fschange.Renamed:
			from, err1 := indexedPath(c.From)
			to, err2 := indexedPath(c.Path)
			if err1 != nil || err2 != nil || from == to {
				continue
			}
			if dir, ok := recordKind(d, from); ok {
				moves = append(moves, move{from: from, to: to, dir: dir})
			}
		}
	}
	return moveRecords(d, moves) || changed
}

// indexedPath is the path the index records for host path p: its directory resolved as
// db.Resolve does (symlinks followed on the part that exists), the name as given. The name is
// never resolved, so a removed or renamed path that no longer exists still maps to its record.
func indexedPath(p string) (string, error) {
	dir, err := db.Resolve(filepath.Dir(p))
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filepath.Base(p)), nil
}

// recordKind reports whether the index has a record at p, and whether it is a directory.
func recordKind(d *db.DB, p string) (dir, ok bool) {
	if _, found, err := db.DirByPath(d, p); err == nil && found {
		return true, true
	}
	_, found, err := d.InstanceID(p)
	return false, err == nil && found
}

// removeRecord removes the directory tree or file record at p; false when the index has none.
func removeRecord(d *db.DB, p string) bool {
	dir, ok := recordKind(d, p)
	if !ok {
		return false
	}
	return retryBusy(func() (err error) {
		if dir {
			_, err = d.RemoveTree(p)
		} else {
			_, err = d.RemoveInstance(p)
		}
		return err
	}) == nil // fs.ErrNotExist: not indexed after all
}

// retryBusy runs fn again while SQLite reports the database busy, up to busyRetries times.
func retryBusy(fn func() error) error {
	err := fn()
	for i := 0; i < busyRetries && isBusy(err); i++ {
		time.Sleep(busyBackoff)
		err = fn()
	}
	return err
}

func isBusy(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "SQLITE_BUSY") || strings.Contains(err.Error(), "database is locked"))
}

// moveRecords moves the records of one batch's renames. They happened together, so a chain
// (a to b with b to c, or a swap) goes through temporary names; otherwise each moves directly.
func moveRecords(d *db.DB, moves []move) (changed bool) {
	froms := make(map[string]bool, len(moves))
	for _, m := range moves {
		froms[m.from] = true
	}
	chained := false
	for _, m := range moves {
		chained = chained || froms[m.to]
	}
	if !chained {
		for _, m := range moves {
			if moveRecord(d, m.from, m.to, m.dir) {
				changed = true
			}
		}
		return changed
	}
	type parked struct {
		move
		temp string
	}
	var park []parked
	for i, m := range moves {
		temp := filepath.Join(filepath.Dir(m.from), fmt.Sprintf("%s%d", tempMovePrefix, i))
		if moveRecord(d, m.from, temp, m.dir) {
			park = append(park, parked{m, temp})
			changed = true
		}
	}
	for _, p := range park {
		if !moveRecord(d, p.temp, p.to, p.dir) {
			moveRecord(d, p.temp, p.from, p.dir) // put it back
		}
	}
	return changed
}

// moveRecord moves the record from from to to. A file replaced at to loses its record first (the
// move would otherwise hit the uniqueness of the instance name); a directory moving onto an
// indexed directory merges, which db.Move does.
func moveRecord(d *db.DB, from, to string, dir bool) bool {
	if !dir {
		if _, found, err := d.InstanceID(to); err == nil && found {
			err := retryBusy(func() error { _, err := d.RemoveInstance(to); return err })
			if err != nil && !isNotIndexed(err) {
				return false
			}
		}
	}
	return retryBusy(func() error { _, err := d.Move(from, to, nil); return err }) == nil
}

func isNotIndexed(err error) bool { return err != nil && errors.Is(err, fs.ErrNotExist) }
