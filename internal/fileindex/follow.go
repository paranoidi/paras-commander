package fileindex

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
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
	// applyPause is the wait after a pass that changed records: the batches arriving meanwhile
	// join one pass, so a job reporting many small changes costs a few panel reloads, not one each.
	applyPause = 200 * time.Millisecond
)

// Apply makes the index follow filesystem changes pc made on disk (fschange.Sink, ADR 0023): a
// removed path loses its record, a renamed one moves its record (a record at the target is
// dropped first, as the rename replaced the file, also when the source has no record to move).
// Paths the index does not know, Created and Dirty changes are ignored: new and changed content
// waits for a scan. It receives every job's changes (ADR 0028), so it stays cheap on the caller's
// side: without a database, or with nothing to follow, nothing is queued, and otherwise it only
// queues the batch; one goroutine applies queued batches in order (it never prompts or creates a
// database). OnChange fires once per pass that changed a record. Until a batch is applied, Overlay
// shows no ghost row for what it took off the disk. A nil Index ignores everything.
func (x *Index) Apply(batch []fschange.Change) {
	if x == nil || len(batch) == 0 || !x.Available() {
		return
	}
	gone := gonePaths(batch)
	if len(gone) == 0 {
		return
	}
	x.holdGone(gone)
	x.applyMu.Lock()
	x.applyPending = append(x.applyPending, queuedChanges{changes: slices.Clone(batch), gone: gone})
	if x.applyDone != nil {
		x.applyMu.Unlock()
		return
	}
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
			x.applyDone = nil
			x.applyMu.Unlock()
			return
		}
		x.applyMu.Unlock()
		changed := false
		if d := x.openQuietly(); d != nil { // otherwise not available (or closed): nothing to follow
			x.writeMu.Lock() // pc's own index writes (Backend.Rename, Remove) do not overlap this
			for _, b := range batches {
				if applyBatch(d, b.changes) {
					changed = true
				}
			}
			x.writeMu.Unlock()
		}
		// After the commit and before OnChange: see goneNow.
		for _, b := range batches {
			x.releaseGone(b.gone)
		}
		if changed {
			x.changed()
			time.Sleep(applyPause)
		}
	}
}

// queuedChanges is one reported batch waiting to be applied, with its gonePaths.
type queuedChanges struct {
	changes []fschange.Change
	gone    []string
}

// gonePaths are the paths a batch took off the disk, as the index records them.
func gonePaths(batch []fschange.Change) []string {
	var paths []string
	for _, c := range batch {
		p := c.Path
		switch c.Kind {
		case fschange.Removed:
		case fschange.Renamed:
			p = c.From
		default:
			continue
		}
		if ip, err := indexedPath(p); err == nil {
			paths = append(paths, ip)
		}
	}
	return paths
}

func (x *Index) holdGone(paths []string) {
	if len(paths) == 0 {
		return
	}
	x.applyMu.Lock()
	defer x.applyMu.Unlock()
	if x.gone == nil {
		x.gone = make(map[string]int, len(paths))
	}
	for _, p := range paths {
		x.gone[p]++
	}
}

func (x *Index) releaseGone(paths []string) {
	if len(paths) == 0 {
		return
	}
	x.applyMu.Lock()
	defer x.applyMu.Unlock()
	for _, p := range paths {
		if x.gone[p]--; x.gone[p] <= 0 {
			delete(x.gone, p)
		}
	}
}

// goneNow is a snapshot of the paths whose change still waits to be applied. A reader of the
// records takes it first: a path released before the snapshot has already lost its record (the
// drain releases after the commit), and one released later is still in the snapshot, so a record
// read afterwards cannot show as a ghost either way.
func (x *Index) goneNow() map[string]int {
	x.applyMu.Lock()
	defer x.applyMu.Unlock()
	if len(x.gone) == 0 {
		return nil
	}
	return maps.Clone(x.gone)
}

// goneAtOrAbove reports whether p or a directory above it is in the goneNow snapshot.
func goneAtOrAbove(gone map[string]int, p string) bool {
	if len(gone) == 0 {
		return false
	}
	for {
		if gone[p] > 0 {
			return true
		}
		parent := filepath.Dir(p)
		if parent == p {
			return false
		}
		p = parent
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

// applyBatch applies one batch of changes in the order they arrived and reports whether a record
// changed. A run of renames applies together, as a chain among them needs (moveRecords). Errors
// leave the records as they were: the next scan puts the index right.
func applyBatch(d *db.DB, batch []fschange.Change) (changed bool) {
	var moves []move
	flush := func() {
		if moveRecords(d, moves) {
			changed = true
		}
		moves = nil
	}
	for _, c := range batch {
		switch c.Kind {
		case fschange.Removed:
			flush()
			if p, err := indexedPath(c.Path); err == nil && removeRecord(d, p, c.IsDir) {
				changed = true
			}
		case fschange.Renamed:
			from, err1 := indexedPath(c.From)
			to, err2 := indexedPath(c.Path)
			if err1 != nil || err2 != nil || from == to {
				continue
			}
			if dir, ok := recordKind(d, from, c.IsDir); ok {
				moves = append(moves, move{from: from, to: to, dir: dir})
			} else if !c.IsDir && !slices.ContainsFunc(moves, func(m move) bool { return m.to == from }) {
				// An unindexed file replaced the target: its record describes content that is gone.
				flush()
				if removeRecord(d, to, false) {
					changed = true
				}
			}
		}
	}
	flush()
	return changed
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

// recordKind reports whether the index has a record at p, and whether it is a directory. It looks
// for the likely kind first: each lookup walks the path, and the change says which it probably is.
func recordKind(d *db.DB, p string, likelyDir bool) (dir, ok bool) {
	isDir := func() bool { _, found, err := db.DirByPath(d, p); return err == nil && found }
	isFile := func() bool { _, found, err := d.InstanceID(p); return err == nil && found }
	if likelyDir {
		if isDir() {
			return true, true
		}
		return false, isFile()
	}
	if isFile() {
		return false, true
	}
	dir = isDir()
	return dir, dir
}

// removeRecord removes the directory tree or file record at p; false when the index has none.
func removeRecord(d *db.DB, p string, likelyDir bool) bool {
	dir, ok := recordKind(d, p, likelyDir)
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
	if !slices.ContainsFunc(moves, func(m move) bool { return froms[m.to] }) {
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
		err := retryBusy(func() error { _, err := d.RemoveInstance(to); return err })
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return false
		}
	}
	move := func() error { return retryBusy(func() error { _, err := d.Move(from, to, nil); return err }) }
	err := move()
	if err == nil {
		return true
	}
	if !dir || isBusy(err) {
		return false
	}
	// rename(2) only lands a directory on an empty one, so records clashing at the target are stale.
	if strings.Contains(err.Error(), "Cannot merge") {
		removed := retryBusy(func() error { _, err := d.RemoveTree(to); return err })
		if removed == nil && move() == nil {
			return true
		}
	}
	// The directory is gone from here whatever the records say: no ghost stays. A scan of the
	// destination records it there.
	return retryBusy(func() error { _, err := d.RemoveTree(from); return err }) == nil
}
