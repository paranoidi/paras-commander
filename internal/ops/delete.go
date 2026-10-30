package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
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
	_, _, err := ExecuteDeletePaths(context.Background(), entryPaths(plan.Entries), nil, nil, nil)
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
// with the deleted path, cumulative file count, and cumulative deleted bytes. onItem, when
// set, reports each path deleted (ItemDone) or skipped after a permission error (ItemSkipped),
// with its index in paths as PlanItem.Source; onSourceStart, when set, reports that index as
// each path's deletion begins.
func ExecuteDeletePaths(ctx context.Context, paths []string, progress func(path string, doneFiles int, doneBytes int64), onItem ItemFunc, onSourceStart func(i int32)) (int, int64, error) {
	var doneFiles int
	var doneBytes int64
	for i, path := range paths {
		if err := ctx.Err(); err != nil {
			return doneFiles, doneBytes, err
		}
		if onSourceStart != nil {
			onSourceStart(int32(i))
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
		rmArgv := []string{"rm", "-f", "--"}
		if ent.Type == fsbackend.EntryDirectory {
			rmArgv = []string{"rm", "-rf", "--"}
		}
		host := hostPathOf(loc)
		err = withElevation(ctx, "delete", host, append(rmArgv, host), func() error {
			return deletePathRecursive(ctx, loc)
		})
		isDir := ent.Type == fsbackend.EntryDirectory
		report := func(res ItemResult, reason string) {
			if onItem != nil {
				onItem(PlanItem{Src: loc, IsDir: isDir, Source: int32(i)}, res, reason)
			}
		}
		if err != nil {
			reportDirty(ctx, loc, isDir) // part of the tree may be gone
		}
		if errors.Is(err, ErrPermissionSkipped) {
			report(ItemSkipped, ReasonPermission)
			continue
		}
		if err != nil {
			if errors.Is(err, ErrUserCanceled) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return doneFiles, doneBytes, err
			}
			return doneFiles, doneBytes, &Error{Op: "delete", Text: "failed to delete " + ent.Name, Err: err}
		}
		reportRemoved(ctx, loc, isDir)
		doneFiles++
		doneBytes += size
		report(ItemDone, "")
		if progress != nil {
			progress(path, doneFiles, doneBytes)
		}
	}
	return doneFiles, doneBytes, nil
}

// deletePathRecursive removes loc and its descendants, checking ctx before each
// child so a cancel stops the current tree instead of finishing it.
func deletePathRecursive(ctx context.Context, loc pathloc.Path) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ent, err := statEntry(ctx, loc)
	if err != nil {
		if isNotExist(err) {
			return nil
		}
		return err
	}
	if ent.Type != fsbackend.EntryDirectory {
		be, err := backendFor(loc)
		if err != nil {
			return err
		}
		return be.Remove(ctx, loc)
	}
	if loc.Scheme() == pathloc.SchemeFile {
		host, err := loc.FilePath()
		if err != nil {
			return err
		}
		return removeLocalTree(ctx, host)
	}
	be, err := backendFor(loc)
	if err != nil {
		return err
	}
	if tr, ok := be.(fsbackend.TreeRemover); ok {
		return tr.RemoveTree(ctx, loc)
	}
	children, err := be.List(ctx, loc)
	if err != nil {
		return err
	}
	for _, child := range children {
		if err := ctx.Err(); err != nil {
			return err
		}
		if child.Name == "." || child.Name == ".." {
			continue
		}
		if err := deletePathRecursive(ctx, child.Loc); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return be.Remove(ctx, loc)
}

// removeLocalTree walks the host tree with os.ReadDir so hidden children are
// removed (backend List hides them) and checks ctx between each child.
func removeLocalTree(ctx context.Context, host string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(host)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return localfs.Remove(host)
	}
	entries, err := os.ReadDir(host)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := removeLocalTree(ctx, filepath.Join(host, e.Name())); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return localfs.Remove(host)
}
