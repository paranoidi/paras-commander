package ops

import (
	"context"
	"fmt"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// DeletePlan describes a validated delete operation.
type DeletePlan struct {
	Entries      []localfs.Entry
	IncludeDirs  bool
	DeleteMode   string // "permanent"
	ConfirmFirst bool   // whether to confirm before executing
}

// DeleteConfirmContent returns the delete confirmation dialog summary line and
// optional recursive-directory warning. Basenames and entry types come from source.Entries.
func DeleteConfirmContent(source Source) (summary, warning string) {
	n := len(source.Entries)
	if n == 1 {
		kind := "file"
		if source.Entries[0].Type == localfs.EntryDirectory {
			kind = "directory"
		}
		return fmt.Sprintf("Delete %s?", kind), ""
	}
	summary = fmt.Sprintf("Delete %d selections?", n)
	dirCount := CountDirectories(source.Entries)
	if dirCount > 0 {
		dirNoun := "directories"
		if dirCount == 1 {
			dirNoun = "directory"
		}
		warning = fmt.Sprintf("Warning: %d %s will be removed recursively!", dirCount, dirNoun)
	}
	return summary, warning
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

// ExecuteDeletePaths deletes the given canonical path strings.
// ctx is checked before each entry. progress is called after each successful deletion
// with the deleted path, cumulative file count, and cumulative deleted bytes.
func ExecuteDeletePaths(ctx context.Context, paths []string, progress func(path string, doneFiles int, doneBytes int64)) (int, int64, error) {
	var doneFiles int
	var doneBytes int64
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return doneFiles, doneBytes, err
		}
		loc, err := pathloc.Parse(path)
		if err != nil {
			return doneFiles, doneBytes, &Error{Op: "delete", Text: "invalid path " + path, Err: err}
		}
		ent, err := statEntry(ctx, loc)
		if err != nil {
			if isNotExist(err) {
				doneFiles++
				if progress != nil {
					progress(path, doneFiles, doneBytes)
				}
				continue
			}
			return doneFiles, doneBytes, &Error{Op: "delete", Text: "failed to stat " + loc.Base(), Err: err}
		}
		size := ent.Size
		if err := removePathRecursive(ctx, loc); err != nil {
			return doneFiles, doneBytes, &Error{Op: "delete", Text: "failed to delete " + ent.Name, Err: err}
		}
		doneFiles++
		doneBytes += size
		if progress != nil {
			progress(path, doneFiles, doneBytes)
		}
	}
	return doneFiles, doneBytes, nil
}
