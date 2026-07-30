package scan

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// Entry is one indexed file or directory under a search root.
type Entry struct {
	Path    string // absolute, clean
	RelLine string // path relative to display root (display / fuzzy match)
	IsDir   bool
	Type    localfs.EntryType
	Size    int64 // file byte size from walk; 0 for directories
}

func buildEntry(root, path string, d fs.DirEntry, isDir bool) (Entry, bool) {
	rel, relErr := filepath.Rel(root, path)
	if relErr != nil {
		return Entry{}, false
	}
	rel = filepath.ToSlash(rel)

	entryType := localfs.EntryFile
	if d.Type()&fs.ModeSymlink != 0 {
		entryType = localfs.EntrySymlink
		if info, statErr := os.Stat(path); statErr == nil {
			isDir = info.IsDir()
		}
	}
	var size int64
	if isDir {
		entryType = localfs.EntryDirectory
	} else if entryType != localfs.EntrySymlink {
		entryType = localfs.EntryFile
		if info, infoErr := d.Info(); infoErr == nil {
			size = info.Size()
		}
	}

	return Entry{
		Path:    filepath.Clean(path),
		RelLine: rel,
		IsDir:   isDir,
		Type:    entryType,
		Size:    size,
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
