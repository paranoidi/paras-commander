package fileindex

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/paranoidi/fileindex/db"
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

// DirDrift reports which of host's indexed subdirectories (names) hold a difference from the
// index anywhere below: a file the scan would add (same rules as the scan), a file whose size
// differs, or an indexed file gone from disk. It walks each subtree, so it runs on its own
// goroutine and stops when ctx is cancelled.
// ponytail: a full walk per subdir and size-only comparison (FilesUnder has no mtime); cache by
// dir mtime or compare mtimes if it proves slow or too coarse.
func (x *Index) DirDrift(ctx context.Context, host string, subdirs []string) (map[string]bool, error) {
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
	out := map[string]bool{}
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
		size := make(map[string]int64, len(recorded))
		for _, e := range recorded {
			size[e.Path] = e.Size
		}
		seen := 0
		for p := range rules.Walk(root) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			want, ok := size[p]
			if !ok {
				out[name] = true
				break
			}
			seen++
			if fi, err := os.Stat(p); err == nil && fi.Size() != want {
				out[name] = true
				break
			}
		}
		if !out[name] && seen < len(recorded) {
			out[name] = true
		}
	}
	return out, nil
}
