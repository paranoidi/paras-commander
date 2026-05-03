package ops

import (
	"os"
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// SymlinkPlan describes a validated symlink creation operation.
type SymlinkPlan struct {
	Target    string // target path (what the link points to)
	LinkPath  string // path to create the link at
	TargetSrc string // display name for the target
}

// PlanSymlink validates a symlink creation operation.
func PlanSymlink(target, linkPath string, activePanelPath string, passivePanelPath string) (SymlinkPlan, error) {
	if target == "" {
		return SymlinkPlan{}, &Error{Op: "symlink", Text: "target path is empty"}
	}
	if linkPath == "" {
		return SymlinkPlan{}, &Error{Op: "symlink", Text: "link path is empty"}
	}

	resolvedTarget := localfs.ResolveRelative(target, activePanelPath)
	resolvedLink := localfs.ResolveRelative(linkPath, passivePanelPath)

	// Check that target exists.
	if _, err := os.Stat(resolvedTarget); err != nil {
		if os.IsNotExist(err) {
			return SymlinkPlan{}, &Error{Op: "symlink", Text: "target does not exist"}
		}
		return SymlinkPlan{}, &Error{Op: "symlink", Text: "cannot access target", Err: err}
	}

	// Check that link path does not already exist.
	if _, err := os.Stat(resolvedLink); err == nil {
		return SymlinkPlan{}, &Error{Op: "symlink", Text: "link path already exists"}
	} else if !os.IsNotExist(err) {
		return SymlinkPlan{}, &Error{Op: "symlink", Text: "cannot stat link path", Err: err}
	}

	targetDisplay := target
	if rel, err := filepath.Rel(activePanelPath, resolvedTarget); err == nil {
		targetDisplay = rel
	}

	return SymlinkPlan{
		Target:    resolvedTarget,
		LinkPath:  resolvedLink,
		TargetSrc: targetDisplay,
	}, nil
}

// ExecuteSymlink creates the symlink.
func ExecuteSymlink(plan SymlinkPlan) error {
	return localfs.Symlink(plan.Target, plan.LinkPath)
}

// HardlinkPlan describes a validated hardlink creation operation.
type HardlinkPlan struct {
	Source     string // source path
	NewPath    string // path to create the link at
	SourceName string // display name
}

// PlanHardlink validates a hardlink creation operation.
func PlanHardlink(source, newPath string, activePanelPath string, passivePanelPath string) (HardlinkPlan, error) {
	if source == "" {
		return HardlinkPlan{}, &Error{Op: "hardlink", Text: "source path is empty"}
	}
	if newPath == "" {
		return HardlinkPlan{}, &Error{Op: "hardlink", Text: "new path is empty"}
	}

	resolvedSource := localfs.ResolveRelative(source, activePanelPath)
	resolvedNewPath := localfs.ResolveRelative(newPath, passivePanelPath)

	// Check that source exists and is not a directory.
	info, err := os.Stat(resolvedSource)
	if err != nil {
		if os.IsNotExist(err) {
			return HardlinkPlan{}, &Error{Op: "hardlink", Text: "source does not exist"}
		}
		return HardlinkPlan{}, &Error{Op: "hardlink", Text: "cannot access source", Err: err}
	}
	if info.IsDir() {
		return HardlinkPlan{}, &Error{Op: "hardlink", Text: "cannot create hardlink to a directory"}
	}

	// Check that new path does not already exist.
	if _, err := os.Stat(resolvedNewPath); err == nil {
		return HardlinkPlan{}, &Error{Op: "hardlink", Text: "destination already exists"}
	} else if !os.IsNotExist(err) {
		return HardlinkPlan{}, &Error{Op: "hardlink", Text: "cannot stat destination", Err: err}
	}

	if filepath.Clean(resolvedSource) == filepath.Clean(resolvedNewPath) {
		return HardlinkPlan{}, &Error{Op: "hardlink", Text: "source and destination are the same"}
	}

	srcName := source
	if rel, err := filepath.Rel(activePanelPath, resolvedSource); err == nil {
		srcName = rel
	}

	return HardlinkPlan{
		Source:     resolvedSource,
		NewPath:    resolvedNewPath,
		SourceName: srcName,
	}, nil
}

// ExecuteHardlink creates the hardlink.
func ExecuteHardlink(plan HardlinkPlan) error {
	return localfs.Link(plan.Source, plan.NewPath)
}
