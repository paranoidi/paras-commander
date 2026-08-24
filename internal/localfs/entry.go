package localfs

import (
	"errors"
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
	// Dev is the entry's st_dev captured from the listing Lstat (DevValid gates it).
	// Mount-boundary checks (panel mount icon) must compare this cached value —
	// never re-stat entry paths at paint time (stat on a copy-saturated NAS mount
	// blocks the UI thread for seconds).
	Dev      uint64
	DevValid bool
	// AccessDenied is derived from the listing Lstat (no re-stat at paint time), same
	// rationale as Dev above.
	AccessDenied bool
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
		return DirectoryListing{}, fmt.Errorf("resolve directory %q: %w", path, pathErrorReason(err))
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return DirectoryListing{}, fmt.Errorf("stat directory %q: %w", cleanPath, pathErrorReason(err))
	}
	if !info.IsDir() {
		return DirectoryListing{}, fmt.Errorf("stat directory %q: not a directory", cleanPath)
	}

	dirEntries, err := os.ReadDir(cleanPath)
	if err != nil {
		return DirectoryListing{}, fmt.Errorf("read directory %q: %w", cleanPath, pathErrorReason(err))
	}

	entries := make([]Entry, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		name := dirEntry.Name()
		isDir := dirEntry.IsDir()
		if !EntryVisible(name, cleanPath, isDir, opts) {
			continue
		}

		entry, keep, err := entryFromDirEntry(cleanPath, dirEntry)
		if err != nil {
			return DirectoryListing{}, err
		}
		if !keep {
			continue
		}
		entries = append(entries, entry)
	}

	return DirectoryListing{Path: cleanPath, Entries: entries}, nil
}

// entryFromDirEntry stats one ReadDir result. When the name vanishes between ReadDir and Info
// (e.g. a concurrent move/delete), keep is false and the caller skips it — aborting the whole
// listing for that race would toast a transient lstat error and leave a stale panel.
func entryFromDirEntry(cleanPath string, dirEntry os.DirEntry) (Entry, bool, error) {
	name := dirEntry.Name()
	path := filepath.Join(cleanPath, name)
	info, err := dirEntry.Info()
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, fs.ErrNotExist) {
			return Entry{}, false, nil
		}
		return Entry{}, false, fmt.Errorf("read metadata for %q: %w", path, err)
	}

	dev, devOK := entryDevice(info)
	return Entry{
		Name:         name,
		Path:         path,
		Type:         classify(info.Mode()),
		Size:         info.Size(),
		Mode:         info.Mode(),
		ModifiedAt:   info.ModTime(),
		Dev:          dev,
		DevValid:     devOK,
		AccessDenied: entryAccessDenied(path, info),
	}, true, nil
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
