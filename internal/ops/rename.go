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

// ExecuteRename performs the rename. A permission failure on a local path is retried as root
// when ctx carries an Elevator (see WithElevator).
func ExecuteRename(ctx context.Context, plan RenamePlan) error {
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
	do := func() error { return be.Rename(ctx, oldLoc, newLoc) }
	if oldHost, newHost := hostPathOf(oldLoc), hostPathOf(newLoc); oldHost != "" && newHost != "" {
		if err := withElevation(ctx, "rename", oldHost, []string{"mv", "-T", "--", oldHost, newHost}, do); err != nil {
			return err
		}
		reportRenamedHost(ctx, oldHost, newHost)
		return nil
	}
	return do()
}
