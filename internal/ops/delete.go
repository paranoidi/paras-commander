package ops

import (
	"context"
	"os"
	"path/filepath"

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
	_, _, err := ExecuteDeletePaths(context.Background(), entryPaths(plan.Entries), nil)
	return err
}

// entryPaths extracts the path strings from a slice of localfs.Entry.
func entryPaths(entries []localfs.Entry) []string {
	paths := make([]string, len(entries))
	for i, e := range entries {
		paths[i] = e.Path
	}
	return paths
}

// ExecuteDeletePaths deletes the given paths, stat-ing each to determine file vs directory.
// ctx is checked before each entry. progress is called after each successful deletion
// with the deleted path, cumulative file count, and cumulative deleted bytes.
func ExecuteDeletePaths(ctx context.Context, paths []string, progress func(path string, doneFiles int, doneBytes int64)) (int, int64, error) {
	var doneFiles int
	var doneBytes int64
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return doneFiles, doneBytes, err
		}
		info, err := os.Lstat(path)
		if err != nil {
			if os.IsNotExist(err) {
				doneFiles++
				if progress != nil {
					progress(path, doneFiles, doneBytes)
				}
				continue
			}
			return doneFiles, doneBytes, &Error{Op: "delete", Text: "failed to stat " + filepath.Base(path), Err: err}
		}
		size := info.Size()
		if info.IsDir() {
			if err := localfs.RemoveAll(path); err != nil {
				return doneFiles, doneBytes, &Error{Op: "delete", Text: "failed to delete directory " + info.Name(), Err: err}
			}
		} else {
			if err := localfs.Remove(path); err != nil {
				// Check if it's actually a directory (e.g., symlink to dir).
				dirInfo, statErr := os.Stat(path)
				if statErr == nil && dirInfo.IsDir() {
					if err := localfs.RemoveAll(path); err != nil {
						return doneFiles, doneBytes, &Error{Op: "delete", Text: "failed to delete " + info.Name(), Err: err}
					}
				} else {
					return doneFiles, doneBytes, &Error{Op: "delete", Text: "failed to delete " + info.Name(), Err: err}
				}
			}
		}
		doneFiles++
		doneBytes += size
		if progress != nil {
			progress(path, doneFiles, doneBytes)
		}
	}
	return doneFiles, doneBytes, nil
}
