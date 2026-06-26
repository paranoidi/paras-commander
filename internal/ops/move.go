package ops

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
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

func countWalkNodesWithProgress(ctx context.Context, root string, baseFiles int, baseBytes int64, srcPath, dstPath string, throttle ProgressEmitThrottle, progress ProgressCallback) (int, error) {
	th := effectiveProgressThrottle(throttle)
	n := 0
	var lastEmit time.Time
	err := localfs.WalkDirRecursive(root, func(path string, info fs.FileInfo) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		_ = path
		_ = info
		n++
		if progress != nil {
			now := time.Now()
			if lastEmit.IsZero() || now.Sub(lastEmit) >= th.MinInterval {
				progress(srcPath, dstPath, baseFiles+n, baseBytes)
				lastEmit = now
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if progress != nil && n > 0 {
		progress(srcPath, dstPath, baseFiles+n, baseBytes)
	}
	return n, nil
}

func countTransferNodesAfterRenameWithProgress(ctx context.Context, dst string, baseFiles int, baseBytes int64, srcPath, dstPath string, throttle ProgressEmitThrottle, progress ProgressCallback) (int, error) {
	loc, err := pathloc.Parse(dst)
	if err != nil {
		return countWalkNodesWithProgress(ctx, dst, baseFiles, baseBytes, srcPath, dstPath, throttle, progress)
	}
	if loc.IsRemote() {
		n, countErr := countTransferNodes(ctx, loc)
		if countErr != nil {
			return 0, countErr
		}
		if progress != nil {
			progress(srcPath, dstPath, baseFiles+n, baseBytes)
		}
		return n, nil
	}
	host, err := loc.FilePath()
	if err != nil {
		return 0, err
	}
	return countWalkNodesWithProgress(ctx, host, baseFiles, baseBytes, srcPath, dstPath, throttle, progress)
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
	if err := removePathRecursive(ctx, dst); err != nil {
		return false, false, false, fmt.Errorf("remove existing %q for overwrite: %w", dst, err)
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
// When plan is non-nil, per-source progress uses pre-scan counts and post-rename walks are skipped.
func executeMoveRenamePhase(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, plan []PlanItem, throttle ProgressEmitThrottle, resolver ConflictResolver, progress ProgressCallback) (doneFiles int, doneBytes int64, fallbackCopy bool, err error) {
	usePlan := len(plan) > 0
	var renamed []renamePair
	var cumulativeFiles int
	var cumulativeBytes int64

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
			pair := renamePair{src.String(), dst.String()}
			renamed = append(renamed, pair)
			if usePlan {
				nf, nb := SummarizePlanForSource(plan, src)
				cumulativeFiles += nf
				cumulativeBytes += nb
				if progress != nil {
					progress(pair.src, pair.dst, cumulativeFiles, cumulativeBytes)
				}
			}
		}
	}

	if usePlan {
		return cumulativeFiles, cumulativeBytes, false, nil
	}

	for _, p := range renamed {
		if err := ctx.Err(); err != nil {
			return 0, 0, false, err
		}
		nf, walkErr := countTransferNodesAfterRenameWithProgress(ctx, p.dst, cumulativeFiles, cumulativeBytes, p.src, p.dst, throttle, progress)
		if walkErr != nil {
			return 0, 0, false, fmt.Errorf("walk after rename %q: %w", p.dst, walkErr)
		}
		cumulativeFiles += nf
	}
	return cumulativeFiles, cumulativeBytes, false, nil
}

// ExecuteMove moves sources to destination using the rename fast path when
// possible for every source, falling back to copy + delete for cross-device moves
// or when any rename in the batch cannot use the fast path.
func ExecuteMove(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}

	doneFiles, doneBytes, fallbackToCopy, err := executeMoveRenamePhase(ctx, sources, destination, nil, throttle, resolver, progress)
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

	doneFiles, doneBytes, transferred, err := executeCopyWithPlan(ctx, plan, sources, destination, opts, throttle, progress, resolver, diskWait)
	if err != nil {
		return doneFiles, doneBytes, fmt.Errorf("move copy phase: %w", err)
	}

	for _, src := range transferred {
		if err := ctx.Err(); err != nil {
			return doneFiles, doneBytes, err
		}
		if err := removePathRecursive(ctx, src); err != nil {
			return doneFiles, doneBytes, fmt.Errorf("move remove source %q: %w", src, err)
		}
	}
	var dirRoots []pathloc.Path
	for _, src := range sources {
		ent, err := statEntry(ctx, src)
		if err != nil {
			continue
		}
		if ent.Type == fsbackend.EntryDirectory {
			dirRoots = append(dirRoots, src)
		}
	}
	if len(dirRoots) > 0 {
		if err := RemoveEmptyDirsUnder(ctx, dirRoots); err != nil {
			return doneFiles, doneBytes, fmt.Errorf("move remove empty source dirs: %w", err)
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

	doneFiles, doneBytes, fallbackToCopy, err := executeMoveRenamePhase(ctx, sources, destination, plan, throttle, resolver, progress)
	if err != nil {
		return 0, 0, err
	}
	if !fallbackToCopy {
		return doneFiles, doneBytes, nil
	}

	return executeMoveCopyPhase(ctx, plan, sources, destination, opts, throttle, progress, resolver, diskWait)
}
