package scan

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// Entry is one indexed file or directory under a search root.
type Entry struct {
	RelLine string // path relative to display root (display / fuzzy match)
	IsDir   bool
	Type    localfs.EntryType
	Size    int64 // file byte size from walk; 0 for directories and lazy pending hidden files
}

func buildEntry(root, path string, d fs.DirEntry, isDir bool) (Entry, bool) {
	rel := relLine(root, path)
	if rel == "" || rel == "." {
		return Entry{}, false
	}

	entryType := localfs.EntryFile
	if d.Type()&fs.ModeSymlink != 0 {
		entryType = localfs.EntrySymlink
		if info, statErr := os.Stat(path); statErr == nil {
			isDir = info.IsDir()
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
		Type:    entryType,
	}, true
}

func entryFromHiddenFilePath(displayRoot, pathOrRel string) (Entry, bool) {
	rel := pathOrRel
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
