package fileindex

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/fileindex/scan"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// Overlay merges the index into a local directory listing for "browse with fileindex": every
// disk entry gets its IndexStatus, and entries only the index knows (deleted since the scan)
// are appended as ghost rows (StatusMissing, a fileindex:// Loc, the indexed size and mtime),
// except what pc itself removed or renamed and Apply has yet to take off the records.
// It does disk and database I/O, so it runs on the listing fetch goroutine. When the index is
// unavailable, or dir is not a local path, entries come back unchanged.
func (x *Index) Overlay(ctx context.Context, dir pathloc.Path, entries []fsbackend.Entry) ([]fsbackend.Entry, error) {
	if dir.Scheme() != pathloc.SchemeFile || !x.Available() {
		return entries, nil
	}
	host, err := indexPath(dir)
	if err != nil {
		return entries, nil
	}
	if raw, err := dir.FilePath(); err == nil && filepath.Clean(raw) != host {
		x.linkedDirs.Store(dir.String(), host)
	}
	d, err := x.DB()
	if err != nil {
		return entries, nil
	}
	panelEntries := make([]localfs.Entry, len(entries))
	names := make([]string, len(entries))
	for i, e := range entries {
		panelEntries[i] = fsbackend.ToPanelEntry(e)
		names[i] = e.Name
	}
	rules, _, _ := x.scanSettings()
	status, err := entryStatuses(ctx, d, host, panelEntries, true, rules)
	if err != nil {
		return entries, err
	}
	tags, err := d.EntryTags(host, names)
	if err != nil {
		return entries, err
	}
	out := make([]fsbackend.Entry, len(entries), len(entries)+8)
	onDisk := make(map[string]bool, len(entries))
	for i, e := range entries {
		onDisk[e.Name] = true
		e.IndexStatus = status[e.Name]
		if e.IndexStatus == StatusIndexed && len(tags[e.Name]) > 0 {
			e.IndexStatus = StatusTagged
		}
		out[i] = e
	}
	gone := x.goneNow() // before the records are read: see goneNow
	files, err := d.DirFiles(host)
	if err != nil {
		return out, err
	}
	// ghostOf adds an index-only name unless it exists on disk after all (hidden or git-ignored
	// entries are missing from entries but not from the disk) or its record is about to go.
	ghostOf := func(name string, mk func(loc pathloc.Path) fsbackend.Entry) {
		if onDisk[name] || goneAtOrAbove(gone, filepath.Join(host, name)) {
			return
		}
		if _, err := os.Lstat(filepath.Join(host, name)); !errors.Is(err, fs.ErrNotExist) {
			return
		}
		loc, err := pathloc.FileIndex(filepath.Join(host, name))
		if err != nil {
			return
		}
		g := mk(loc)
		g.IndexStatus = StatusMissing
		out = append(out, g)
	}
	for name, r := range files {
		if ctx.Err() != nil {
			return entries, ctx.Err()
		}
		ghostOf(name, func(loc pathloc.Path) fsbackend.Entry {
			return fileEntry(loc, db.ListedFile{Name: name, Size: r.Size, Mtime: r.Mtime})
		})
	}
	if id, found, err := db.DirByPath(d, host); err == nil && found {
		subdirs, err := subdirNames(d, id)
		if err != nil {
			return out, err
		}
		for _, name := range subdirs {
			ghostOf(name, func(loc pathloc.Path) fsbackend.Entry { return dirEntry(loc, name) })
		}
	}
	return out, nil
}

// ShownDir is the directory a panel at dir shows, as the index records it, to compare with OnChange's
// and OnScanCommit's directories; "" when dir is neither local nor fileindex://. It does no I/O, so it
// suits the UI goroutine: a path through a symlink is known from the panel's last Overlay.
func (x *Index) ShownDir(dir pathloc.Path) string {
	switch dir.Scheme() {
	case pathloc.SchemeFile:
		if host, ok := x.linkedDirs.Load(dir.String()); ok {
			return host.(string)
		}
		raw, err := dir.FilePath()
		if err != nil {
			return ""
		}
		return filepath.Clean(raw)
	case pathloc.SchemeFileIndex:
		host, err := pathloc.IndexHostPath(dir)
		if err != nil {
			return ""
		}
		return filepath.Clean(host)
	}
	return ""
}

// DirDrift checks host's indexed subdirectories (names) against the index, one after the other, and
// hands each to yield as its walk finishes, with IndexStatusDiffers (driftWalk found a difference: a file the scan would add under the same rules,
// a file whose size or mtime differs, or a recorded file that Stat says is gone; unhashed files
// count as indexed), IndexStatusUnchecked (nothing differs within maxDepth, but a directory lies
// deeper) or IndexStatusIndexed. maxDepth counts directory levels below the subdirectory; 0 means
// unlimited (never unchecked). Only recorded files within the depth are compared, and a recorded
// file the walk does not reach (under an excluded directory, in an unreadable one, or no longer a
// regular file) differs only when it is really gone. It walks each subtree, so it runs on its own
// goroutine and stops with an error when ctx is cancelled; a name the index does not have is skipped.
// ponytail: a full walk per subdir; cache by dir mtime if it proves slow. The depth-limited walk
// re-implements scan.Rules.Walk's file filters; share them if fileindex exports a depth option.
func (x *Index) DirDrift(ctx context.Context, host string, subdirs []string, maxDepth int, yield func(name, status string)) error {
	if len(subdirs) == 0 {
		return nil
	}
	d, err := x.DB()
	if err != nil {
		return err
	}
	rules, _, err := x.scanSettings()
	if err != nil {
		return err
	}
	for _, name := range subdirs {
		differs := false
		deeper, err := driftWalk(ctx, d, rules, filepath.Join(host, name), maxDepth, nil, func(Difference) bool {
			differs = true
			return false
		})
		if errors.Is(err, ErrNotIndexed) {
			continue
		}
		if err != nil {
			return err
		}
		switch {
		case differs:
			yield(name, StatusDiffers)
		case deeper:
			yield(name, StatusUnchecked)
		default:
			yield(name, StatusIndexed)
		}
	}
	return nil
}

// ErrNotIndexed is returned by driftWalk (and the differences task) for a directory the index does
// not have.
var ErrNotIndexed = errors.New("not in the index")

// Difference states: how a path under an indexed directory differs from the index.
const (
	DiffNew     = "new"     // on disk, included by the scan rules, not in the index
	DiffChanged = "changed" // in the index with another size or modification time
	DiffMissing = "missing" // in the index, gone from disk
)

// Difference is one path where the disk and the index disagree.
type Difference struct {
	Path  string
	State string // DiffNew, DiffChanged or DiffMissing
}

// driftWalk compares the files under the indexed directory root (host path, as the index records
// it) with the index and yields each difference, stopping when yield returns false. It is the one
// source of "differs" for the directory state icon (DirDrift) and the differences report.
//
// The walk applies the scan rules (the files a scan would add) down to maxDepth directory levels
// below root (0 = unlimited) and reports deeper whether it skipped a directory below that limit.
// A walked file is new when the index has no record of it and changed when it is recorded with
// another size or mtime (Recorded.Same, as a file row's status). Once the walk has finished (not
// when yield stopped it), every recorded file within the depth that the walk did not visit is
// checked on its own: it differs (missing) only when Stat says it is gone, so a record under an
// excluded or unreadable directory, or one that is no longer a regular file, is no difference.
// onFile, when not nil, is called with each walked file's path. ErrNotIndexed means root is not in
// the index.
func driftWalk(ctx context.Context, d *db.DB, rules scan.Rules, root string, maxDepth int, onFile func(string), yield func(Difference) bool) (deeper bool, err error) {
	id, found, err := db.DirByPath(d, root)
	if err != nil {
		return false, err
	}
	if !found || id == nil {
		return false, ErrNotIndexed
	}
	recorded, err := d.FilesUnder(*id, true)
	if err != nil {
		return false, err
	}
	want := make(map[string]db.Recorded, len(recorded))
	for _, e := range recorded {
		if maxDepth == 0 || dirDepth(root, e.Path) <= maxDepth {
			want[e.Path] = db.Recorded{Size: e.Size, Mtime: e.Mtime}
		}
	}
	stopped := false
	visit := func(p string, fi fs.FileInfo) bool {
		if ctx.Err() != nil {
			return false
		}
		if onFile != nil {
			onFile(p)
		}
		rec, ok := want[p]
		if !ok {
			stopped = !yield(Difference{Path: p, State: DiffNew})
			return !stopped
		}
		delete(want, p) // what stays in want is what the walk did not reach
		// size and mtime only, as a file row's changed status; an unhashed file is pending work, not
		// drift.
		if !rec.Same(fi.Size(), fi.ModTime()) {
			stopped = !yield(Difference{Path: p, State: DiffChanged})
			return !stopped
		}
		return true
	}
	if maxDepth == 0 {
		for p, fi := range rules.WalkDirs(root, nil) {
			if !visit(p, fi) {
				break
			}
		}
	} else {
		deeper = walkToDepth(rules, root, maxDepth, visit)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if stopped {
		return deeper, nil
	}
	for p := range want {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if gone(p) && !yield(Difference{Path: p, State: DiffMissing}) {
			break
		}
	}
	return deeper, nil
}

// gone reports whether Stat finds p missing, as fileindex's own prune decides (db.missing): only a
// missing path, a missing directory above it or a symlink loop count, not an access error.
func gone(p string) bool {
	_, err := os.Stat(p)
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) || errors.Is(err, syscall.ELOOP)
}

// dirDepth is the number of directory levels file lies below root.
func dirDepth(root, file string) int {
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return 0
	}
	return strings.Count(rel, string(filepath.Separator))
}

// walkToDepth yields the files rules include in root and its directories down to maxDepth
// levels (the file filters of scan.Rules.Walk), stopping when yield returns false. It reports
// whether it skipped a directory below the limit.
func walkToDepth(rules scan.Rules, root string, maxDepth int, yield func(string, fs.FileInfo) bool) (deeper bool) {
	visited := map[scan.DirKey]bool{}
	if fi, err := os.Stat(root); err == nil {
		if k, ok := scan.KeyOf(fi); ok {
			visited[k] = true // a link back to root is not entered again, as in scan.Rules.WalkDirs
		}
	}
	var walk func(dir string, depth int) bool
	walk = func(dir string, depth int) bool {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return true
		}
		var subdirs []string
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			isDir, viaLink := e.IsDir(), false
			if e.Type()&fs.ModeSymlink != 0 {
				if t, err := os.Stat(p); err == nil && t.IsDir() {
					isDir, viaLink = true, true
				}
			}
			if isDir {
				if !rules.ExcludeDir(e.Name()) && (!viaLink || rules.FollowSymlinks) {
					subdirs = append(subdirs, p)
				}
				continue
			}
			if !rules.IncludeFile(e.Name()) {
				continue
			}
			if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() && !yield(p, fi) {
				return false
			}
		}
		for _, sub := range subdirs {
			if depth == maxDepth {
				deeper = true
				continue
			}
			if fi, err := os.Stat(sub); err == nil {
				if k, ok := scan.KeyOf(fi); ok {
					if visited[k] {
						continue
					}
					visited[k] = true
				}
			}
			if !walk(sub, depth+1) {
				return false
			}
		}
		return true
	}
	walk(root, 0)
	return deeper
}
