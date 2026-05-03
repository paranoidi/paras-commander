package ops

import (
	"os"
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// MkdirPlan describes a validated mkdir operation.
type MkdirPlan struct {
	Path string // absolute path to create
	Name string // display name
}

// PlanMkdir validates a mkdir operation.
//
// - Input is resolved relative to the panel path.
// - Parent directories are not created; simple os.Mkdir semantics.
func PlanMkdir(input string, panelPath string) (MkdirPlan, error) {
	if input == "" {
		return MkdirPlan{}, &Error{Op: "mkdir", Text: "directory name is empty"}
	}

	dest := localfs.ResolveRelative(input, panelPath)

	// Check if something already exists at the target path.
	if _, err := os.Stat(dest); err == nil {
		return MkdirPlan{}, &Error{Op: "mkdir", Text: "target already exists"}
	} else if !os.IsNotExist(err) {
		return MkdirPlan{}, &Error{Op: "mkdir", Text: "cannot stat target", Err: err}
	}

	name := input
	if rel, err := filepath.Rel(panelPath, dest); err == nil {
		name = rel
	}

	return MkdirPlan{
		Path: dest,
		Name: name,
	}, nil
}

// ExecuteMkdir creates the directory with default permissions.
func ExecuteMkdir(plan MkdirPlan) error {
	return localfs.Mkdir(plan.Path, 0o755)
}
