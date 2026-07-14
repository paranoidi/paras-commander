package ops

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// DuplicatePlan describes a validated same-directory directory copy.
type DuplicatePlan struct {
	SourcePath string
	NewName    string
	DestPath   string
}

// ValidateDuplicateSource requires exactly one entry (file or directory,
// selection or cursor).
func ValidateDuplicateSource(p *panel.State) (localfs.Entry, error) {
	source, err := ResolveSource(p)
	if err != nil {
		return localfs.Entry{}, err
	}
	if len(source.Entries) > 1 {
		return localfs.Entry{}, &Error{Op: "duplicate", Text: "select a single file or directory"}
	}
	return source.Entries[0], nil
}

// PlanDuplicate validates a same-directory copy of a directory under a new basename.
func PlanDuplicate(source localfs.Entry, newName string, panelPath string) (DuplicatePlan, error) {
	trimmed := strings.TrimSpace(newName)
	if trimmed == "" {
		return DuplicatePlan{}, &Error{Op: "duplicate", Text: "name is empty"}
	}
	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, string(filepath.Separator)) {
		return DuplicatePlan{}, &Error{Op: "duplicate", Text: "name must be a single filename without path separators"}
	}
	if trimmed == "." || trimmed == ".." {
		return DuplicatePlan{}, &Error{Op: "duplicate", Text: "invalid name"}
	}

	srcLoc, err := pathloc.Parse(source.Path)
	if err != nil {
		return DuplicatePlan{}, &Error{Op: "duplicate", Text: "invalid source path", Err: err}
	}
	if srcLoc.Base() == trimmed {
		return DuplicatePlan{}, &Error{Op: "duplicate", Text: "new name must differ from the original"}
	}

	parent := srcLoc.Parent()
	if parent.IsZero() {
		panel, perr := pathloc.Parse(panelPath)
		if perr != nil {
			return DuplicatePlan{}, &Error{Op: "duplicate", Text: "invalid panel path", Err: perr}
		}
		parent = panel
	}
	destLoc, err := parent.Join(trimmed)
	if err != nil {
		return DuplicatePlan{}, &Error{Op: "duplicate", Text: err.Error(), Err: err}
	}

	if _, err := statEntry(context.Background(), destLoc); err == nil {
		return DuplicatePlan{}, &Error{Op: "duplicate", Text: "target already exists"}
	} else if !isNotExist(err) {
		return DuplicatePlan{}, &Error{Op: "duplicate", Text: "cannot stat target", Err: err}
	}

	return DuplicatePlan{
		SourcePath: srcLoc.String(),
		NewName:    trimmed,
		DestPath:   destLoc.String(),
	}, nil
}
