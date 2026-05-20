package ops

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// MkdirPlan describes a validated mkdir operation.
type MkdirPlan struct {
	Path string // canonical pathloc string
	Name string // display name
}

// PlanMkdir validates a mkdir operation.
//
// - Input is resolved relative to the panel path.
// - Parent directories are not created; single-level Mkdir semantics.
func PlanMkdir(input string, panelPath string) (MkdirPlan, error) {
	if strings.TrimSpace(input) == "" {
		return MkdirPlan{}, &Error{Op: "mkdir", Text: "directory name is empty"}
	}

	parent, err := pathloc.Parse(panelPath)
	if err != nil {
		return MkdirPlan{}, &Error{Op: "mkdir", Text: "invalid panel path", Err: err}
	}
	dest, err := resolveChild(parent, input)
	if err != nil {
		return MkdirPlan{}, &Error{Op: "mkdir", Text: err.Error(), Err: err}
	}

	if _, err := statEntry(context.Background(), dest); err == nil {
		return MkdirPlan{}, &Error{Op: "mkdir", Text: "target already exists"}
	} else if !isNotExist(err) {
		return MkdirPlan{}, &Error{Op: "mkdir", Text: "cannot stat target", Err: err}
	}

	name := input
	if !parent.IsRemote() {
		if host, err := parent.FilePath(); err == nil {
			if destHost, err := dest.FilePath(); err == nil {
				if rel, err := filepath.Rel(host, destHost); err == nil {
					name = rel
				}
			}
		}
	} else if !strings.Contains(strings.TrimSpace(input), "/") {
		name = strings.TrimSpace(input)
	}

	return MkdirPlan{
		Path: dest.String(),
		Name: name,
	}, nil
}

// ExecuteMkdir creates the directory with default permissions.
func ExecuteMkdir(plan MkdirPlan) error {
	loc, err := pathloc.Parse(plan.Path)
	if err != nil {
		return err
	}
	be, err := backendFor(loc)
	if err != nil {
		return err
	}
	if loc.IsRemote() {
		return be.Mkdir(context.Background(), loc, 0o755)
	}
	host, err := loc.FilePath()
	if err != nil {
		return err
	}
	return localfs.Mkdir(host, 0o755)
}
