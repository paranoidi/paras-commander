package ops

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// MovePlanTotals returns a file/byte estimate consistent with the copy fallback path.
// Rename fast path does not read bytes; totals still give a useful upper bound for UI.
func MovePlanTotals(sources []pathloc.Path, destination pathloc.Path) (totalFiles int, totalBytes int64, err error) {
	return CopyPlanTotals(sources, destination)
}

type renamePair struct {
	src, dst string
}

func renamePairsRollback(pairs []renamePair) {
	for i := len(pairs) - 1; i >= 0; i-- {
		srcLoc, err1 := pathloc.Parse(pairs[i].src)
		dstLoc, err2 := pathloc.Parse(pairs[i].dst)
		if err1 == nil && err2 == nil && srcLoc.IsRemote() {
			if be, err := backendFor(dstLoc); err == nil {
				_ = be.Rename(context.Background(), dstLoc, srcLoc)
			}
			continue
		}
		_ = os.Rename(pairs[i].dst, pairs[i].src)
	}
}

func countTransferNodesAfterRename(dst string) (int, error) {
	loc, err := pathloc.Parse(dst)
	if err != nil {
		return countWalkNodes(dst)
	}
	if loc.IsRemote() {
		return countTransferNodes(context.Background(), loc)
	}
	host, err := loc.FilePath()
	if err != nil {
		return 0, err
	}
	return countWalkNodes(host)
}

func countWalkNodes(root string) (int, error) {
	n := 0
	err := localfs.WalkDirRecursive(root, func(string, fs.FileInfo) error {
		n++
		return nil
	})
	return n, err
}

// renameSourceForMove handles conflict resolution then RenameFastPath for one source.
// Returns renamed when the path was moved, skipped when the user chose not to overwrite,
// fallbackCopy when cross-device (or non-fast) rename requires copy+delete for the batch.
func renameSourceForMove(ctx context.Context, src, dst pathloc.Path, resolver ConflictResolver) (renamed, skipped, fallbackCopy bool, err error) {
	if err := ctx.Err(); err != nil {
		return false, false, false, err
	}
	_, statErr := statEntry(ctx, dst)
	if isNotExist(statErr) {
		return renameFastPathOrFallback(src, dst)
	}
	if statErr != nil {
		return false, false, false, fmt.Errorf("stat destination %q: %w", dst, statErr)
	}
	if resolver == nil {
		return false, false, false, fmt.Errorf("destination %q already exists and no conflict resolver configured", dst)
	}
	facts, err := statConflictFacts(ctx, src, dst)
	if err != nil {
		return false, false, false, fmt.Errorf("conflict stat %q %q: %w", src, dst, err)
	}
	overwrite, err := resolver(src.String(), dst.String(), facts)
	if err != nil {
		return false, false, false, err
	}
	if !overwrite {
		return false, true, false, nil
	}
	return renameFastPathOrFallback(src, dst)
}

func renameFastPathOrFallback(src, dst pathloc.Path) (renamed, skipped, fallbackCopy bool, err error) {
	ok, err := RenameFastPath(src, dst)
	if err != nil {
		return false, false, false, err
	}
	if !ok {
		return false, false, true, nil
	}
	return true, false, false, nil
}

// executeMoveRenamePhase tries rename for each source with conflict checks.
// When fallbackCopy is true, prior renames in this batch were rolled back.
func executeMoveRenamePhase(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, resolver ConflictResolver, progress ProgressCallback) (doneFiles int, doneBytes int64, fallbackCopy bool, err error) {
	var renamed []renamePair
	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			renamePairsRollback(renamed)
			return 0, 0, false, err
		}
		dst := ResolveDestination(src, destination)
		didRename, skipped, needCopy, renameErr := renameSourceForMove(ctx, src, dst, resolver)
		if renameErr != nil {
			renamePairsRollback(renamed)
			return 0, 0, false, fmt.Errorf("rename %q -> %q: %w", src, dst, renameErr)
		}
		if needCopy {
			renamePairsRollback(renamed)
			return 0, 0, true, nil
		}
		if skipped {
			continue
		}
		if didRename {
			renamed = append(renamed, renamePair{src.String(), dst.String()})
		}
	}
	cumulative := 0
	for _, p := range renamed {
		if err := ctx.Err(); err != nil {
			return 0, 0, false, err
		}
		nf, walkErr := countTransferNodesAfterRename(p.dst)
		if walkErr != nil {
			return 0, 0, false, fmt.Errorf("walk after rename %q: %w", p.dst, walkErr)
		}
		cumulative += nf
		if progress != nil {
			progress(p.src, p.dst, cumulative, 0)
		}
	}
	return cumulative, 0, false, nil
}

// ExecuteMove moves sources to destination using the rename fast path when
// possible for every source, falling back to copy + delete for cross-device moves
// or when any rename in the batch cannot use the fast path.
func ExecuteMove(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}

	doneFiles, doneBytes, fallbackToCopy, err := executeMoveRenamePhase(ctx, sources, destination, resolver, progress)
	if err != nil {
		return 0, 0, err
	}
	if !fallbackToCopy {
		return doneFiles, doneBytes, nil
	}

	return executeMoveCopyPhase(ctx, nil, sources, destination, opts, throttle, progress, resolver, diskWait)
}

func executeMoveCopyPhase(ctx context.Context, planOptional []PlanItem, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	var plan []PlanItem
	var tb int64
	var planErr error
	if planOptional != nil {
		plan = planOptional
		_, _, tb = SummarizePlan(plan)
	} else {
		plan, _, _, tb, planErr = BuildCopyPlanWithTotalsCtx(ctx, sources, destination, PlanBuildOptions{})
		if planErr != nil {
			return 0, 0, fmt.Errorf("move copy phase plan: %w", planErr)
		}
	}
	if err := EnsureDiskSpace(diskWait, destination, tb, pathloc.Path{}); err != nil {
		return 0, 0, err
	}

	doneFiles, doneBytes, err := ExecuteCopyUsingPlan(ctx, plan, sources, destination, opts, throttle, progress, resolver, diskWait)
	if err != nil {
		return doneFiles, doneBytes, fmt.Errorf("move copy phase: %w", err)
	}

	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			return doneFiles, doneBytes, err
		}
		if err := removePathRecursive(ctx, src); err != nil {
			return doneFiles, doneBytes, fmt.Errorf("move remove source %q: %w", src, err)
		}
	}

	return doneFiles, doneBytes, nil
}

// ExecuteMoveWithPlan tries the rename fast path, then uses plan for the copy+delete fallback without rebuilding it.
func ExecuteMoveWithPlan(ctx context.Context, plan []PlanItem, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	if plan == nil {
		return ExecuteMove(ctx, sources, destination, opts, throttle, progress, resolver, diskWait)
	}
	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}

	doneFiles, doneBytes, fallbackToCopy, err := executeMoveRenamePhase(ctx, sources, destination, resolver, progress)
	if err != nil {
		return 0, 0, err
	}
	if !fallbackToCopy {
		return doneFiles, doneBytes, nil
	}

	return executeMoveCopyPhase(ctx, plan, sources, destination, opts, throttle, progress, resolver, diskWait)
}
