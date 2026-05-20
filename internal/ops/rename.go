package ops

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// RenamePlan describes a validated same-directory rename.
type RenamePlan struct {
	SourcePath string // current canonical path
	NewName    string // new basename
	NewPath    string // new canonical path
}

// PlanRename validates an in-place rename operation.
//
// - newName must be a single filename (no path separators).
// - source must exist.
// - newName must differ from the current basename.
func PlanRename(source localfs.Entry, newName string, panelPath string) (RenamePlan, error) {
	if newName == "" {
		return RenamePlan{}, &Error{Op: "rename", Text: "name is empty"}
	}
	if strings.Contains(newName, "/") || strings.Contains(newName, string(filepath.Separator)) {
		return RenamePlan{}, &Error{Op: "rename", Text: "name must be a single filename without path separators"}
	}

	srcLoc, err := pathloc.Parse(source.Path)
	if err != nil {
		return RenamePlan{}, &Error{Op: "rename", Text: "invalid source path", Err: err}
	}
	if srcLoc.Base() == newName {
		return RenamePlan{}, &Error{Op: "rename", Text: "new name is the same as the current name"}
	}

	parent := srcLoc.Parent()
	if parent.IsZero() {
		panel, err := pathloc.Parse(panelPath)
		if err != nil {
			return RenamePlan{}, &Error{Op: "rename", Text: "invalid panel path", Err: err}
		}
		parent = panel
	}
	newLoc, err := parent.Join(newName)
	if err != nil {
		return RenamePlan{}, &Error{Op: "rename", Text: err.Error(), Err: err}
	}

	if _, err := statEntry(context.Background(), newLoc); err == nil {
		return RenamePlan{}, &Error{Op: "rename", Text: "target already exists"}
	} else if !isNotExist(err) {
		return RenamePlan{}, &Error{Op: "rename", Text: "cannot stat target", Err: err}
	}

	return RenamePlan{
		SourcePath: srcLoc.String(),
		NewName:    newName,
		NewPath:    newLoc.String(),
	}, nil
}

// ExecuteRename performs the rename.
func ExecuteRename(plan RenamePlan) error {
	oldLoc, err := pathloc.Parse(plan.SourcePath)
	if err != nil {
		return err
	}
	newLoc, err := pathloc.Parse(plan.NewPath)
	if err != nil {
		return err
	}
	be, err := backendFor(oldLoc)
	if err != nil {
		return err
	}
	return be.Rename(context.Background(), oldLoc, newLoc)
}
