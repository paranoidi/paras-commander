package ops

import (
	"os"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// DeletePlan describes a validated delete operation.
type DeletePlan struct {
	Entries      []localfs.Entry
	IncludeDirs  bool
	DeleteMode   string // "permanent"
	ConfirmFirst bool   // whether to confirm before executing
}

// PlanDelete validates a delete operation.
//
// - Source must have at least one entry.
// - Directories are removed recursively (with appropriate warning).
// - Delete mode must be "permanent" (the only v1 option).
func PlanDelete(source Source, confirmDelete bool, deleteMode string) (DeletePlan, error) {
	if len(source.Entries) == 0 {
		return DeletePlan{}, &Error{Op: "delete", Text: "no entries to delete"}
	}
	if deleteMode == "" {
		deleteMode = "permanent"
	}
	if deleteMode != "permanent" {
		return DeletePlan{}, &Error{Op: "delete", Text: "unsupported delete mode: " + deleteMode}
	}

	includeDirs := false
	for _, e := range source.Entries {
		if e.Type == localfs.EntryDirectory {
			includeDirs = true
			break
		}
	}

	return DeletePlan{
		Entries:      source.Entries,
		IncludeDirs:  includeDirs,
		DeleteMode:   deleteMode,
		ConfirmFirst: confirmDelete,
	}, nil
}

// ExecuteDelete performs the deletion.
// Directories are removed recursively.
func ExecuteDelete(plan DeletePlan) error {
	for _, entry := range plan.Entries {
		if entry.Type == localfs.EntryDirectory {
			if err := localfs.RemoveAll(entry.Path); err != nil {
				return &Error{Op: "delete", Text: "failed to delete directory " + entry.Name, Err: err}
			}
		} else {
			if err := localfs.Remove(entry.Path); err != nil {
				// Check if it's actually a directory (e.g., symlink to dir).
				info, statErr := os.Stat(entry.Path)
				if statErr == nil && info.IsDir() {
					if err := localfs.RemoveAll(entry.Path); err != nil {
						return &Error{Op: "delete", Text: "failed to delete " + entry.Name, Err: err}
					}
				} else {
					return &Error{Op: "delete", Text: "failed to delete " + entry.Name, Err: err}
				}
			}
		}
	}
	return nil
}
