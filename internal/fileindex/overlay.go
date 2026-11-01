package fileindex

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/fileindex/scan"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// Overlay merges the index into a local directory listing for "browse with fileindex": every
// disk entry gets its IndexStatus, and entries only the index knows (deleted since the scan)
// are appended as ghost rows (StatusMissing, a fileindex:// Loc, the indexed size and mtime).
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
	files, err := d.DirFiles(host)
	if err != nil {
		return out, err
	}
	// ghostOf adds an index-only name unless it exists on disk after all (hidden or git-ignored
	// entries are missing from entries but not from the disk).
	ghostOf := func(name string, mk func(loc pathloc.Path) fsbackend.Entry) {
		if onDisk[name] {
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

// DirDrift checks each of host's indexed subdirectories (names) against the index and returns
// IndexStatusDiffers (a file the scan would add under the same rules, a file whose size or
// mtime differs, or an indexed file gone from disk), IndexStatusUnchecked (nothing differs within maxDepth, but
// a directory lies deeper) or IndexStatusIndexed. maxDepth counts directory levels below the
// subdirectory; 0 means unlimited (never unchecked). Only recorded files within the depth are
// compared. It walks each subtree, so it runs on its own goroutine and stops when ctx is
// cancelled.
// ponytail: a full walk per subdir; cache by dir mtime if it proves slow. The depth-limited walk
// re-implements scan.Rules.Walk's file filters; share them if fileindex exports a depth option.
func (x *Index) DirDrift(ctx context.Context, host string, subdirs []string, maxDepth int) (map[string]string, error) {
	if len(subdirs) == 0 {
		return nil, nil
	}
	d, err := x.DB()
	if err != nil {
		return nil, err
	}
	rules, _, err := x.scanSettings()
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, name := range subdirs {
		root := filepath.Join(host, name)
		id, found, err := db.DirByPath(d, root)
		if err != nil {
			return nil, err
		}
		if !found || id == nil {
			continue
		}
		recorded, err := d.FilesUnder(*id, true)
		if err != nil {
			return nil, err
		}
		want := make(map[string]db.Recorded, len(recorded))
		for _, e := range recorded {
			if maxDepth == 0 || dirDepth(root, e.Path) <= maxDepth {
				want[e.Path] = db.Recorded{Size: e.Size, Mtime: e.Mtime}
			}
		}
		differs, seen, deeper := false, 0, false
		visit := func(p string) bool {
			if ctx.Err() != nil {
				return false
			}
			rec, ok := want[p]
			if !ok {
				differs = true
				return false
			}
			seen++
			// the same comparison as a file row's status (entryStatuses)
			if fi, err := os.Stat(p); err == nil && !rec.Unchanged(fi.Size(), fi.ModTime()) {
				differs = true
				return false
			}
			return true
		}
		if maxDepth == 0 {
			for p := range rules.Walk(root) {
				if !visit(p) {
					break
				}
			}
		} else {
			deeper = walkToDepth(rules, root, maxDepth, visit)
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		switch {
		case differs || seen < len(want):
			out[name] = StatusDiffers
		case deeper:
			out[name] = StatusUnchecked
		default:
			out[name] = StatusIndexed
		}
	}
	return out, nil
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
func walkToDepth(rules scan.Rules, root string, maxDepth int, yield func(string) bool) (deeper bool) {
	visited := map[scan.DirKey]bool{}
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
			if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() && !yield(p) {
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
