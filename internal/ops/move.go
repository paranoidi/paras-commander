package ops

import (
	"context"
	"errors"
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
	// staged is the sibling path holding the original destination when this rename
	// overwrote an existing dest. Empty when dest did not exist.
	staged string
}

func renamePairsRollback(pairs []renamePair) error {
	var errs []error
	for i := len(pairs) - 1; i >= 0; i-- {
		if err := rollbackRenamePair(pairs[i]); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func rollbackRenamePair(p renamePair) error {
	srcLoc, err1 := pathloc.Parse(p.src)
	dstLoc, err2 := pathloc.Parse(p.dst)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("rollback parse %q -> %q: %v; %v", p.src, p.dst, err1, err2)
	}
	var renameErr error
	if srcLoc.IsRemote() {
		if be, err := backendFor(dstLoc); err == nil {
			renameErr = be.Rename(context.Background(), dstLoc, srcLoc)
		} else {
			renameErr = err
		}
	} else {
		renameErr = os.Rename(p.dst, p.src)
	}
	if renameErr != nil {
		return fmt.Errorf("rollback rename %q -> %q: %w", p.dst, p.src, renameErr)
	}
	if p.staged == "" {
		return nil
	}
	stagedLoc, err := pathloc.Parse(p.staged)
	if err != nil {
		return fmt.Errorf("rollback parse staged %q: %w", p.staged, err)
	}
	return restoreStagedDest(context.Background(), dstLoc, stagedLoc)
}

func rollbackRenames(pairs []renamePair, err error) error {
	if rbErr := renamePairsRollback(pairs); rbErr != nil {
		return fmt.Errorf("%w (rollback: %v)", err, rbErr)
	}
	return err
}

func stageExistingDest(ctx context.Context, dst pathloc.Path) (pathloc.Path, error) {
	staged, err := pickMoveStashSibling(ctx, dst)
	if err != nil {
		return pathloc.Path{}, err
	}
	ok, err := RenameFastPath(dst, staged)
	if err != nil {
		return pathloc.Path{}, fmt.Errorf("stage existing %q: %w", dst, err)
	}
	if !ok {
		return pathloc.Path{}, fmt.Errorf("stage existing %q: cannot rename aside for overwrite", dst)
	}
	return staged, nil
}

func pickMoveStashSibling(ctx context.Context, dst pathloc.Path) (pathloc.Path, error) {
	parent := dst.Parent()
	for i := 0; i < 1_000_000; i++ {
		cand, err := parent.Join(fmt.Sprintf(".paras-move-stash-%d", i))
		if err != nil {
			return pathloc.Path{}, err
		}
		_, statErr := statEntry(ctx, cand)
		if isNotExist(statErr) {
			return cand, nil
		}
		if statErr != nil {
			return pathloc.Path{}, statErr
		}
	}
	return pathloc.Path{}, fmt.Errorf("could not allocate stash name for %q", dst)
}

func restoreStagedDest(ctx context.Context, dst, staged pathloc.Path) error {
	ok, err := RenameFastPath(staged, dst)
	if err != nil {
		return fmt.Errorf("restore staged %q -> %q: %w", staged, dst, err)
	}
	if !ok {
		return fmt.Errorf("restore staged %q -> %q: cannot rename", staged, dst)
	}
	return nil
}

func discardStagedDests(ctx context.Context, pairs []renamePair) {
	for _, p := range pairs {
		if p.staged == "" {
			continue
		}
		loc, err := pathloc.Parse(p.staged)
		if err != nil {
			continue
		}
		_ = removePathRecursive(ctx, loc)
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
// staged is the parked original destination after an overwrite; empty otherwise.
func renameSourceForMove(ctx context.Context, src, dst pathloc.Path, resolver ConflictResolver) (renamed, skipped, fallbackCopy bool, staged string, err error) {
	if err := ctx.Err(); err != nil {
		return false, false, false, "", err
	}
	_, statErr := statEntry(ctx, dst)
	if isNotExist(statErr) {
		renamed, skipped, fallbackCopy, err = renameFastPathOrFallback(src, dst)
		return renamed, skipped, fallbackCopy, "", err
	}
	if statErr != nil {
		return false, false, false, "", fmt.Errorf("stat destination %q: %w", dst, statErr)
	}
	if resolver == nil {
		return false, false, false, "", fmt.Errorf("destination %q already exists and no conflict resolver configured", dst)
	}
	facts, err := statConflictFacts(ctx, src, dst)
	if err != nil {
		return false, false, false, "", fmt.Errorf("conflict stat %q %q: %w", src, dst, err)
	}
	overwrite, err := resolver(src.String(), dst.String(), facts)
	if err != nil {
		return false, false, false, "", err
	}
	if !overwrite {
		return false, true, false, "", nil
	}
	stagedLoc, stageErr := stageExistingDest(ctx, dst)
	if stageErr != nil {
		return false, false, false, "", stageErr
	}
	renamed, skipped, fallbackCopy, err = renameFastPathOrFallback(src, dst)
	if err != nil || fallbackCopy || skipped || !renamed {
		if restoreErr := restoreStagedDest(ctx, dst, stagedLoc); restoreErr != nil {
			if err != nil {
				return false, skipped, fallbackCopy, "", fmt.Errorf("%w (restore staged dest: %v)", err, restoreErr)
			}
			return false, skipped, fallbackCopy, "", restoreErr
		}
		return false, skipped, fallbackCopy, "", err
	}
	return true, false, false, stagedLoc.String(), nil
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
			return 0, 0, false, rollbackRenames(renamed, err)
		}
		name := TransferDestName(src, nameRoot)
		dst := ResolveDestinationNamed(destination, name)
		if strings.ContainsAny(name, `/\`) {
			if err := ensureParentDirs(ctx, dst); err != nil {
				return 0, 0, false, rollbackRenames(renamed, fmt.Errorf("create parent for %q: %w", dst, err))
			}
		}
		didRename, skipped, needCopy, staged, renameErr := renameSourceForMove(ctx, src, dst, resolver)
		if renameErr != nil {
			return 0, 0, false, rollbackRenames(renamed, fmt.Errorf("rename %q -> %q: %w", src, dst, renameErr))
		}
		if needCopy {
			if rbErr := renamePairsRollback(renamed); rbErr != nil {
				return 0, 0, false, fmt.Errorf("move rename fallback rollback: %w", rbErr)
			}
			return 0, 0, true, nil
		}
		if skipped {
			continue
		}
		if didRename {
			pair := renamePair{src: src.String(), dst: dst.String(), staged: staged}
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

	discardStagedDests(ctx, renamed)

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
		plan, _, _, tb, planErr = BuildCopyPlanWithTotalsCtx(ctx, sources, destination, PlanBuildOptions{FlatDestNames: opts.FlatDestNames, DereferenceSymlinks: opts.DereferenceSymlinks})
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
	return finishMoveCopyPhase(ctx, sources, transferred, doneFiles, doneBytes, opts.OnRemoveSources)
}

// finishMoveCopyPhase removes transferred sources and any now-empty source directory roots
// after a move's copy-fallback phase has copied everything to destination. Shared by the
// slice-backed (executeMoveCopyPhase) and channel-backed (ExecuteMoveWithPlanChan) fallback
// phases so this tail logic has one source of truth. onRemove, when non-nil, is called once
// before the removal loop begins.
func finishMoveCopyPhase(ctx context.Context, sources []pathloc.Path, transferred []pathloc.Path, doneFiles int, doneBytes int64, onRemove func()) (int, int64, error) {
	if onRemove != nil {
		onRemove()
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
// BuildPlanStreamCtx). The rename fast path must not start until that delivery walk has finished
// reading the source paths: renaming a directory while WalkDirRecursive still ReadDir's it
// produces self-inflicted enumeration errors and an incomplete plan for mixed-device fallback.
func ExecuteMoveWithPlanChan(ctx context.Context, planCh <-chan PlanItem, planErr func() error, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}

	plan, err := collectPlanChan(ctx, planCh, planErr)
	if err != nil {
		return 0, 0, err
	}
	drainPlanChanDiscard(ctx, planCh)
	return ExecuteMoveWithPlan(ctx, plan, sources, destination, opts, throttle, progress, resolver, diskWait)
}

// collectPlanChan drains planCh until it closes and then returns planErr() (if any). A move's
// rename phase uses the collected plan only after this returns, so source paths stay stable
// for the duration of the delivery walk.
func collectPlanChan(ctx context.Context, planCh <-chan PlanItem, planErr func() error) ([]PlanItem, error) {
	if planCh == nil {
		if planErr != nil {
			return nil, planErr()
		}
		return nil, nil
	}
	var plan []PlanItem
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case it, ok := <-planCh:
			if !ok {
				if planErr != nil {
					if err := planErr(); err != nil {
						return plan, err
					}
				}
				return plan, nil
			}
			plan = append(plan, it)
		}
	}
}
