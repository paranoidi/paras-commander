package ops

import (
	"os"
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// RenamePlan describes a validated same-directory rename.
type RenamePlan struct {
	SourcePath string // current absolute path
	NewName    string // new basename
	NewPath    string // new absolute path
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
	if filepath.Base(newName) != newName {
		return RenamePlan{}, &Error{Op: "rename", Text: "name must be a single filename without path separators"}
	}

	// If the source is a non-absolute path, resolve it relative to the panel.
	sourcePath := source.Path
	if !filepath.IsAbs(sourcePath) {
		sourcePath = filepath.Join(panelPath, sourcePath)
	}
	sourcePath = filepath.Clean(sourcePath)

	curBase := filepath.Base(sourcePath)
	if curBase == newName {
		return RenamePlan{}, &Error{Op: "rename", Text: "new name is the same as the current name"}
	}

	newPath := filepath.Clean(filepath.Join(filepath.Dir(sourcePath), newName))

	// Check if target already exists.
	if _, err := os.Stat(newPath); err == nil {
		return RenamePlan{}, &Error{Op: "rename", Text: "target already exists"}
	} else if !os.IsNotExist(err) {
		return RenamePlan{}, &Error{Op: "rename", Text: "cannot stat target", Err: err}
	}

	return RenamePlan{
		SourcePath: sourcePath,
		NewName:    newName,
		NewPath:    newPath,
	}, nil
}

// ExecuteRename performs the rename.
func ExecuteRename(plan RenamePlan) error {
	return localfs.Rename(plan.SourcePath, plan.NewPath)
}
