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

// ExecuteMove moves sources to destination using the rename fast path when
// possible for every source, falling back to copy + delete for cross-device moves
// or when any rename in the batch cannot use the fast path.
func ExecuteMove(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}

	var renamed []renamePair
	fallbackToCopy := false
	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			renamePairsRollback(renamed)
			return 0, 0, err
		}
		dst := ResolveDestination(src, destination)
		ok, err := RenameFastPath(src, dst)
		if err != nil {
			renamePairsRollback(renamed)
			return 0, 0, fmt.Errorf("rename %q -> %q: %w", src, dst, err)
		}
		if !ok {
			renamePairsRollback(renamed)
			fallbackToCopy = true
			break
		}
		renamed = append(renamed, renamePair{src.String(), dst.String()})
	}

	if !fallbackToCopy && len(renamed) == len(sources) {
		cumulative := 0
		for _, p := range renamed {
			if err := ctx.Err(); err != nil {
				return 0, 0, err
			}
			nf, err := countTransferNodesAfterRename(p.dst)
			if err != nil {
				return 0, 0, fmt.Errorf("walk after rename %q: %w", p.dst, err)
			}
			cumulative += nf
			if progress != nil {
				progress(p.src, p.dst, cumulative, 0)
			}
		}
		return cumulative, 0, nil
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

	var renamed []renamePair
	fallbackToCopy := false
	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			renamePairsRollback(renamed)
			return 0, 0, err
		}
		dst := ResolveDestination(src, destination)
		ok, err := RenameFastPath(src, dst)
		if err != nil {
			renamePairsRollback(renamed)
			return 0, 0, fmt.Errorf("rename %q -> %q: %w", src, dst, err)
		}
		if !ok {
			renamePairsRollback(renamed)
			fallbackToCopy = true
			break
		}
		renamed = append(renamed, renamePair{src.String(), dst.String()})
	}

	if !fallbackToCopy && len(renamed) == len(sources) {
		cumulative := 0
		for _, p := range renamed {
			if err := ctx.Err(); err != nil {
				return 0, 0, err
			}
			nf, err := countTransferNodesAfterRename(p.dst)
			if err != nil {
				return 0, 0, fmt.Errorf("walk after rename %q: %w", p.dst, err)
			}
			cumulative += nf
			if progress != nil {
				progress(p.src, p.dst, cumulative, 0)
			}
		}
		return cumulative, 0, nil
	}

	return executeMoveCopyPhase(ctx, plan, sources, destination, opts, throttle, progress, resolver, diskWait)
}
