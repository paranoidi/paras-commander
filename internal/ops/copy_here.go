package ops

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// CopyHerePlan describes a validated same-directory directory copy.
type CopyHerePlan struct {
	SourcePath string
	NewName    string
	DestPath   string
}

// ValidateCopyHereSource requires exactly one directory (selection or cursor).
func ValidateCopyHereSource(p *panel.State) (localfs.Entry, error) {
	source, err := ResolveSource(p)
	if err != nil {
		return localfs.Entry{}, err
	}
	if len(source.Entries) > 1 {
		return localfs.Entry{}, &Error{Op: "copy-here", Text: "select a single directory"}
	}
	entry := source.Entries[0]
	if entry.Type != localfs.EntryDirectory {
		return localfs.Entry{}, &Error{Op: "copy-here", Text: "not a directory"}
	}
	return entry, nil
}

// PlanCopyHere validates a same-directory copy of a directory under a new basename.
func PlanCopyHere(source localfs.Entry, newName string, panelPath string) (CopyHerePlan, error) {
	trimmed := strings.TrimSpace(newName)
	if trimmed == "" {
		return CopyHerePlan{}, &Error{Op: "copy-here", Text: "name is empty"}
	}
	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, string(filepath.Separator)) {
		return CopyHerePlan{}, &Error{Op: "copy-here", Text: "name must be a single filename without path separators"}
	}
	if trimmed == "." || trimmed == ".." {
		return CopyHerePlan{}, &Error{Op: "copy-here", Text: "invalid name"}
	}

	srcLoc, err := pathloc.Parse(source.Path)
	if err != nil {
		return CopyHerePlan{}, &Error{Op: "copy-here", Text: "invalid source path", Err: err}
	}
	if srcLoc.Base() == trimmed {
		return CopyHerePlan{}, &Error{Op: "copy-here", Text: "new name must differ from the original"}
	}

	parent := srcLoc.Parent()
	if parent.IsZero() {
		panel, perr := pathloc.Parse(panelPath)
		if perr != nil {
			return CopyHerePlan{}, &Error{Op: "copy-here", Text: "invalid panel path", Err: perr}
		}
		parent = panel
	}
	destLoc, err := parent.Join(trimmed)
	if err != nil {
		return CopyHerePlan{}, &Error{Op: "copy-here", Text: err.Error(), Err: err}
	}

	if _, err := statEntry(context.Background(), destLoc); err == nil {
		return CopyHerePlan{}, &Error{Op: "copy-here", Text: "target already exists"}
	} else if !isNotExist(err) {
		return CopyHerePlan{}, &Error{Op: "copy-here", Text: "cannot stat target", Err: err}
	}

	return CopyHerePlan{
		SourcePath: srcLoc.String(),
		NewName:    trimmed,
		DestPath:   destLoc.String(),
	}, nil
}
