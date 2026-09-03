package ops

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
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
func executeMoveRenamePhase(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, plan []PlanItem, flatNames bool, throttle ProgressEmitThrottle, resolver ConflictResolver, progress ProgressCallback) (doneFiles int, doneBytes int64, fallbackCopy bool, err error) {
	usePlan := len(plan) > 0
	var renamed []renamePair
	var cumulativeFiles int
	var cumulativeBytes int64

	var nameRoot pathloc.Path
	if !flatNames {
		nameRoot = TransferNameRoot(sources)
	}
	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			renamePairsRollback(renamed)
			return 0, 0, false, err
		}
		name := TransferDestName(src, nameRoot)
		dst := ResolveDestinationNamed(destination, name)
		if strings.ContainsAny(name, `/\`) {
			if err := ensureParentDirs(ctx, dst); err != nil {
				renamePairsRollback(renamed)
				return 0, 0, false, fmt.Errorf("create parent for %q: %w", dst, err)
			}
		}
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

	doneFiles, doneBytes, fallbackToCopy, err := executeMoveRenamePhase(ctx, sources, destination, nil, opts.FlatDestNames, throttle, resolver, progress)
	if err != nil {
		return 0, 0, err
	}
	if !fallbackToCopy {
		return doneFiles, doneBytes, nil
	}

	return transferRun{
		ctx: ctx, sources: sources, destination: destination, opts: opts,
		throttle: throttle, progress: progress, resolver: resolver, diskWait: diskWait,
	}.executeMoveCopyPhase()
}

func executeMoveCopyPhase(ctx context.Context, planOptional []PlanItem, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	var plan []PlanItem
	var tb int64
	var planErr error
	if planOptional != nil {
		plan = planOptional
		_, _, tb = SummarizePlan(plan)
	} else {
		plan, _, _, tb, planErr = BuildCopyPlanWithTotalsCtx(ctx, sources, destination, PlanBuildOptions{FlatDestNames: opts.FlatDestNames})
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
	return finishMoveCopyPhase(ctx, sources, transferred, doneFiles, doneBytes)
}

// finishMoveCopyPhase removes transferred sources and any now-empty source directory roots
// after a move's copy-fallback phase has copied everything to destination. Shared by the
// slice-backed (executeMoveCopyPhase) and channel-backed (ExecuteMoveWithPlanChan) fallback
// phases so this tail logic has one source of truth.
func finishMoveCopyPhase(ctx context.Context, sources []pathloc.Path, transferred []pathloc.Path, doneFiles int, doneBytes int64) (int, int64, error) {
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

	doneFiles, doneBytes, fallbackToCopy, err := executeMoveRenamePhase(ctx, sources, destination, plan, opts.FlatDestNames, throttle, resolver, progress)
	if err != nil {
		return 0, 0, err
	}
	if !fallbackToCopy {
		return doneFiles, doneBytes, nil
	}

	return transferRun{
		ctx: ctx, sources: sources, destination: destination, opts: opts,
		throttle: throttle, progress: progress, resolver: resolver, diskWait: diskWait,
		planOptional: plan,
	}.executeMoveCopyPhase()
}

// ExecuteMoveWithPlanChan mirrors ExecuteMoveWithPlan but consumes a streamed plan channel (from
// BuildPlanStreamCtx) for its copy-fallback phase instead of a pre-built slice, so a cross-device
// move's transfer can start before the whole source tree is enumerated. The rename fast path
// (executeMoveRenamePhase) renames each top-level source directly and never needs the plan, so it
// runs exactly as ExecuteMove's does — planCh is only consumed once a fallback is actually needed.
func ExecuteMoveWithPlanChan(ctx context.Context, planCh <-chan PlanItem, planErr func() error, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}

	doneFiles, doneBytes, fallbackToCopy, err := executeMoveRenamePhase(ctx, sources, destination, nil, opts.FlatDestNames, throttle, resolver, progress)
	if err != nil {
		return 0, 0, err
	}
	if !fallbackToCopy {
		// ponytail: rename fast path never touches the plan; drain and discard whatever the
		// background producer still has in flight so its goroutine can finish and exit instead
		// of blocking forever on a channel nobody reads. Bails out early on ctx cancellation —
		// state.go's cancelJobScan backstops the producer's own context in that case.
		drainPlanChanDiscard(ctx, planCh)
		return doneFiles, doneBytes, nil
	}

	// No upfront EnsureDiskSpace(tb) here: total bytes aren't known until the streamed plan
	// finishes. The per-file EnsureDiskSpace check inside copyRegularItem is the safety net,
	// same trade-off jobbridge makes for streaming copy jobs (see llm-docs/jobs.md).
	copyFiles, copyBytes, transferred, err := executeCopyIter(ctx, planIterChan(ctx, planCh, planErr), destination, opts, throttle, progress, resolver, diskWait)
	if err != nil {
		return copyFiles, copyBytes, fmt.Errorf("move copy phase: %w", err)
	}
	return finishMoveCopyPhase(ctx, sources, transferred, copyFiles, copyBytes)
}
