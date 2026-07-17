package ops

import (
	"errors"
	"fmt"
	"os"

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
		return fmt.Sprintf("%s: %s (%s)", e.Op, e.Text, nestedErrorText(e.Err))
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Text)
}

// NestedErrorText returns a short underlying reason without repeated paths from remove wrappers.
func NestedErrorText(err error) string {
	return nestedErrorText(err)
}

// PathErrorReason returns pathErr.Err when err wraps *os.PathError.
func PathErrorReason(err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && pathErr.Err != nil {
		return pathErr.Err
	}
	return err
}

func nestedErrorText(err error) string {
	if err == nil {
		return ""
	}
	if reason := PathErrorReason(err); reason != err {
		return reason.Error()
	}
	if u := errors.Unwrap(err); u != nil {
		if tail := nestedErrorText(u); tail != "" {
			return tail
		}
	}
	return err.Error()
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
		entries, err := p.SelectedEntries(false, func(path string) (localfs.Entry, error) {
			e, err := entryFromPathString(path)
			if err != nil {
				return localfs.Entry{}, &Error{Op: "source", Text: fmt.Sprintf("selected path %q: %v", path, err), Err: err}
			}
			return e, nil
		})
		if err != nil {
			return Source{}, err
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
