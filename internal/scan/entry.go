package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/findcrit"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

// Entry is one indexed file or directory under a search root.
type Entry struct {
	RelLine string // path relative to display root (display / fuzzy match)
	IsDir   bool
	// Broken is set on a symlink whose target does not resolve.
	Broken bool
	Type   localfs.EntryType
	Size   int64 // file byte size; 0 unless the walk stats (StartOpts.Stat) and for directories
	// Meta is the lstat metadata when the walk stats (StartOpts.Stat >= findcrit.StatMeta).
	Meta *findcrit.Meta
}

// subject is what findcrit.Criteria reads of the entry.
func (e Entry) subject() findcrit.Subject {
	return findcrit.Subject{
		Name:    e.RelLine[strings.LastIndexByte(e.RelLine, '/')+1:],
		IsDir:   e.IsDir,
		Symlink: e.Type == localfs.EntrySymlink,
		Broken:  e.Broken,
		Size:    e.Size,
		Meta:    e.Meta,
	}
}

// entryStat is the metadata a statting walk collected for one directory child.
type entryStat struct {
	meta *findcrit.Meta
	size int64
}

// statEntry lstats path for a walk at level (nil below findcrit.StatMeta or on error); with
// findcrit.StatMetaEmptyDirs a directory is also opened to learn whether it has entries.
func statEntry(path string, level findcrit.StatLevel) *entryStat {
	if level < findcrit.StatMeta {
		return nil
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil
	}
	st := &entryStat{meta: findcrit.MetaFromInfo(info)}
	switch {
	case info.Mode().IsRegular():
		st.size = info.Size()
	case info.IsDir() && level >= findcrit.StatMetaEmptyDirs:
		st.meta.EmptyDir = dirIsEmpty(path)
	}
	return st
}

// dirIsEmpty reports whether dir can be read and has no entries.
func dirIsEmpty(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	names, err := f.Readdirnames(1)
	return len(names) == 0 && err != nil
}

// withStat attaches st (from statEntry) to e.
func (e Entry) withStat(st *entryStat) Entry {
	if st != nil {
		e.Meta = st.meta
		if !e.IsDir {
			e.Size = st.size
		}
	}
	return e
}

func buildEntry(root, path string, d fs.DirEntry, isDir bool) (Entry, bool) {
	rel := relLine(root, path)
	if rel == "" || rel == "." {
		return Entry{}, false
	}

	entryType := localfs.EntryFile
	broken := false
	if d.Type()&fs.ModeSymlink != 0 {
		entryType = localfs.EntrySymlink
		if info, statErr := os.Stat(path); statErr == nil {
			isDir = info.IsDir()
		} else {
			broken = true
		}
	}
	if isDir {
		entryType = localfs.EntryDirectory
	} else if entryType != localfs.EntrySymlink {
		entryType = localfs.EntryFile
	}

	return Entry{
		RelLine: rel,
		IsDir:   isDir,
		Broken:  broken,
		Type:    entryType,
	}, true
}

func entryFromHiddenFilePath(displayRoot, pathOrRel string) (Entry, bool) {
	var rel string
	if filepath.IsAbs(pathOrRel) {
		rel = relLine(displayRoot, pathOrRel)
	} else {
		rel = filepath.ToSlash(pathOrRel)
	}
	if rel == "" {
		return Entry{}, false
	}
	return Entry{
		RelLine: rel,
		Type:    localfs.EntryFile,
	}, true
}

func relLine(displayRoot, absPath string) string {
	displayRoot = filepath.Clean(displayRoot)
	absPath = filepath.Clean(absPath)
	if absPath == displayRoot {
		return "."
	}
	prefix := displayRoot + string(filepath.Separator)
	if len(absPath) > len(prefix) && absPath[:len(prefix)] == prefix {
		return filepath.ToSlash(absPath[len(prefix):])
	}
	if rel, err := filepath.Rel(displayRoot, absPath); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(absPath)
}
