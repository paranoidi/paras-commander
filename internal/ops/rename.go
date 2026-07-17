package ops

import (
	"context"

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
	srcLoc, newLoc, err := planSiblingTarget(source.Path, panelPath, newName, siblingTargetRules{
		op:           "rename",
		sameNameText: "new name is the same as the current name",
		requireUTF8:  true,
	})
	if err != nil {
		return RenamePlan{}, err
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
