package ops

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// MkdirPlan describes a validated mkdir operation.
type MkdirPlan struct {
	Path string // canonical pathloc string
	Name string // display name
	// Parents creates missing parent directories too (mkdir -p). Local paths only.
	Parents bool
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
	if parent.IsLocal() {
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

// ExecuteMkdir creates the directory with default permissions. A permission failure on a local
// path is retried as root when ctx carries an Elevator (the new directory is then root-owned).
func ExecuteMkdir(ctx context.Context, plan MkdirPlan) error {
	loc, err := pathloc.Parse(plan.Path)
	if err != nil {
		return err
	}
	be, err := backendFor(loc)
	if err != nil {
		return err
	}
	if !loc.IsLocal() {
		return be.Mkdir(ctx, loc, 0o755)
	}
	host, err := loc.FilePath()
	if err != nil {
		return err
	}
	if plan.Parents {
		return mkdirParents(ctx, host)
	}
	if err := withElevation(ctx, "mkdir", host, []string{"mkdir", "--", host}, func() error {
		return localfs.Mkdir(host, 0o755)
	}); err != nil {
		return err
	}
	reportCreatedHost(ctx, host)
	return nil
}

// mkdirParents creates host and its missing parents and reports the topmost directory it
// created (everything below it is new too).
func mkdirParents(ctx context.Context, host string) error {
	top := topmostMissing(host)
	if err := withElevation(ctx, "mkdir", host, []string{"mkdir", "-p", "--", host}, func() error {
		return localfs.MkdirAll(host, 0o755)
	}); err != nil {
		return err
	}
	if top != "" {
		reportCreatedHost(ctx, top)
	}
	return nil
}

// topmostMissing returns the highest ancestor of host (host included) that does not exist, or
// "" when host exists.
func topmostMissing(host string) string {
	top := ""
	for p := filepath.Clean(host); ; p = filepath.Dir(p) {
		if _, err := os.Lstat(p); err == nil {
			return top
		}
		top = p
		if filepath.Dir(p) == p {
			return top
		}
	}
}
