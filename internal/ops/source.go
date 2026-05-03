package ops

import (
	"fmt"
	"sort"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
)

// SourceKind classifies the source of a file operation.
type SourceKind int

const (
	SourceCursor   SourceKind = iota // Single entry under cursor
	SourceSelected                   // One or more selected entries
)

// Source describes what to operate on.
type Source struct {
	Kind    SourceKind
	Entries []localfs.Entry // non-empty
}

// Error is a typed operation error with a user-facing message.
type Error struct {
	Op   string // short operation name, e.g. "rename"
	Text string // human-readable message
	Err  error  // underlying error, may be nil
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Op, e.Text, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Text)
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.Err
}

// SourceError returns a typed error for source-related failures.
func SourceError(text string) *Error {
	return &Error{Op: "source", Text: text}
}

// ResolveSource determines the operation source from panel state.
//
// Rules:
//   - If the active panel has selected files in the current directory, operate on the selection.
//   - Otherwise operate on the cursor entry.
//   - Empty panels or no valid cursor entry produce an error.
func ResolveSource(p *panel.State) (Source, error) {
	if len(p.SelectedPaths) > 0 {
		byPath := make(map[string]localfs.Entry, len(p.Entries))
		for _, entry := range p.Entries {
			byPath[entry.Path] = entry
		}
		paths := make([]string, 0, len(p.SelectedPaths))
		for path := range p.SelectedPaths {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		entries := make([]localfs.Entry, 0, len(paths))
		for _, path := range paths {
			if e, ok := byPath[path]; ok {
				entries = append(entries, e)
				continue
			}
			e, err := localfs.EntryFromPath(path)
			if err != nil {
				return Source{}, &Error{Op: "source", Text: fmt.Sprintf("selected path %q: %v", path, err), Err: err}
			}
			entries = append(entries, e)
		}
		if len(entries) > 0 {
			return Source{Kind: SourceSelected, Entries: entries}, nil
		}
	}

	// Fall back to cursor entry.
	entry, ok := p.CurrentEntry()
	if !ok {
		return Source{}, SourceError("no entries in panel")
	}
	return Source{Kind: SourceCursor, Entries: []localfs.Entry{entry}}, nil
}

// ResolveSourceSingle returns the single source entry for operations that only
// support one source (rename, symlink source, hardlink source).
// If there are selected entries, it returns an error.
func ResolveSourceSingle(p *panel.State) (localfs.Entry, error) {
	if len(p.SelectedPaths) > 0 {
		return localfs.Entry{}, SourceError("single-entry operation but multiple files are selected")
	}
	entry, ok := p.CurrentEntry()
	if !ok {
		return localfs.Entry{}, SourceError("no entries in panel")
	}
	return entry, nil
}

// SourcePaths returns source entry paths in panel/listing order.
func SourcePaths(source Source) []string {
	paths := make([]string, 0, len(source.Entries))
	for _, entry := range source.Entries {
		paths = append(paths, entry.Path)
	}
	return paths
}

// CountDirectories returns the number of directory-type entries in the slice.
func CountDirectories(entries []localfs.Entry) int {
	count := 0
	for _, e := range entries {
		if e.Type == localfs.EntryDirectory {
			count++
		}
	}
	return count
}
