package ops

import (
	"context"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// DeletePlan describes a validated delete operation.
type DeletePlan struct {
	Entries      []localfs.Entry
	IncludeDirs  bool
	ConfirmFirst bool // whether to confirm before executing
}

// PlanDelete validates a delete operation.
//
// - Source must have at least one entry.
// - Directories are removed recursively (with appropriate warning).
func PlanDelete(source Source, confirmDelete bool) (DeletePlan, error) {
	if len(source.Entries) == 0 {
		return DeletePlan{}, &Error{Op: "delete", Text: "no entries to delete"}
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
				return doneFiles, doneBytes, &Error{Op: "delete", Text: loc.Base() + " does not exist", Err: err}
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
