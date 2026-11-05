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
// when ctx carries an Elevator (see WithElevator). A fileindex:// rename of an entry that
// exists on disk renames it there and refuses a target that exists on disk, whether the index
// lists it or not (the index follows through the fschange sink); an entry gone from disk is moved
// in the index only.
func ExecuteRename(ctx context.Context, plan RenamePlan) error {
	oldLoc, err := pathloc.Parse(plan.SourcePath)
	if err != nil {
		return err
	}
	newLoc, err := pathloc.Parse(plan.NewPath)
	if err != nil {
		return err
	}
	if oldLoc, newLoc, err = renameOnDisk(ctx, oldLoc, newLoc); err != nil {
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

// renameOnDisk translates a fileindex:// rename of an entry on disk to the real paths. The index
// lists indexed files only, so the planner's target check cannot see an unindexed file that
// rename(2) would silently replace: the target is checked on disk here.
func renameOnDisk(ctx context.Context, oldLoc, newLoc pathloc.Path) (pathloc.Path, pathloc.Path, error) {
	if oldLoc.Scheme() != pathloc.SchemeFileIndex || newLoc.Scheme() != pathloc.SchemeFileIndex {
		return oldLoc, newLoc, nil
	}
	local := OnDisk(oldLoc)
	if local.Scheme() == pathloc.SchemeFileIndex {
		return oldLoc, newLoc, nil
	}
	target := newLoc.Local()
	if _, err := statEntry(ctx, target); err == nil {
		return oldLoc, newLoc, &Error{Op: "rename", Text: "target already exists"}
	} else if !isNotExist(err) {
		return oldLoc, newLoc, &Error{Op: "rename", Text: "cannot stat target", Err: err}
	}
	return local, target, nil
}
