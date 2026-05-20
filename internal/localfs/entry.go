package localfs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/paranoidi/paras-commander/internal/gitignore"
)

// EntryType is the file kind the UI needs for Phase 1 rendering.
type EntryType int

const (
	EntryFile EntryType = iota
	EntryDirectory
	EntrySymlink
	EntryOther
)

// Entry is a normalized directory entry independent of terminal rendering.
type Entry struct {
	Name       string
	Path       string
	Type       EntryType
	Size       int64
	Mode       fs.FileMode
	ModifiedAt time.Time
}

// ListOptions controls directory listing behavior.
type ListOptions struct {
	ShowHidden bool
	Gitignore  *gitignore.Matcher // nil = no gitignore filtering
}

// DirectoryListing is a normalized local directory snapshot.
type DirectoryListing struct {
	Path    string
	Entries []Entry
}

// DefaultListOptions returns built-in listing defaults.
func DefaultListOptions() ListOptions {
	return ListOptions{
		ShowHidden: false,
	}
}

// ListDir reads and normalizes a local directory listing.
func ListDir(path string, opts ListOptions) (DirectoryListing, error) {
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return DirectoryListing{}, fmt.Errorf("resolve directory %q: %w", path, err)
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return DirectoryListing{}, fmt.Errorf("stat directory %q: %w", cleanPath, err)
	}
	if !info.IsDir() {
		return DirectoryListing{}, fmt.Errorf("stat directory %q: not a directory", cleanPath)
	}

	dirEntries, err := os.ReadDir(cleanPath)
	if err != nil {
		return DirectoryListing{}, fmt.Errorf("read directory %q: %w", cleanPath, err)
	}

	entries := make([]Entry, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		name := dirEntry.Name()
		isDir := dirEntry.IsDir()
		if !EntryVisible(name, cleanPath, isDir, opts) {
			continue
		}

		info, err := dirEntry.Info()
		if err != nil {
			return DirectoryListing{}, fmt.Errorf("read metadata for %q: %w", filepath.Join(cleanPath, name), err)
		}

		entries = append(entries, Entry{
			Name:       name,
			Path:       filepath.Join(cleanPath, name),
			Type:       classify(info.Mode()),
			Size:       info.Size(),
			Mode:       info.Mode(),
			ModifiedAt: info.ModTime(),
		})
	}

	return DirectoryListing{Path: cleanPath, Entries: entries}, nil
}

// IsDir returns true if the entry is a directory.
func (e Entry) IsDir() bool {
	return e.Type == EntryDirectory
}

func classify(mode fs.FileMode) EntryType {
	switch {
	case mode.IsDir():
		return EntryDirectory
	case mode&fs.ModeSymlink != 0:
		return EntrySymlink
	case mode.IsRegular():
		return EntryFile
	default:
		return EntryOther
	}
}
