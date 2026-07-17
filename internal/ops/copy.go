package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// PlanBuildOptions configures optional hooks during copy/move plan walks.
type PlanBuildOptions struct {
	// OnPath is invoked for each visited source path during directory walks (and top-level sources).
	OnPath func(path string) error
	// YieldEveryN invokes Yield after every N walk callbacks when Yield is non-nil.
	YieldEveryN int
	Yield       func()
	// FlatDestNames mirrors Options.FlatDestNames for plan building.
	FlatDestNames bool
}

// ConflictResolver is called when a destination path already exists.
// It receives source and destination paths plus file metadata and returns true to overwrite,
// false to skip, and an error to abort.
type ConflictResolver func(src, dest string, facts FileConflictFacts) (overwrite bool, err error)

// BuildCopyPlanWithTotals prepares the destination (when needed), walks sources, and returns the flat plan plus totals.
func BuildCopyPlanWithTotals(sources []pathloc.Path, destination pathloc.Path) (plan []PlanItem, totalFiles int, totalBytes int64, err error) {
	p, tf, _, tb, err := BuildCopyPlanWithTotalsCtx(context.Background(), sources, destination, PlanBuildOptions{})
	return p, tf, tb, err
}

// BuildCopyPlanWithTotalsCtx is like BuildCopyPlanWithTotals but honors ctx cancellation and plan walk hooks.
func BuildCopyPlanWithTotalsCtx(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, opts PlanBuildOptions) (plan []PlanItem, totalItems, totalDirs int, totalBytes int64, err error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, 0, 0, err
	}
	// Resolve destinations before prepareCopyDestination: mkdir for a single-directory
	// copy to a new sibling name must not make ResolveDestination treat dest as a container.
	p, err := BuildPlanCtx(ctx, sources, destination, true, opts)
	if err != nil {
		return nil, 0, 0, 0, fmt.Errorf("build copy plan: %w", err)
	}
	if err := prepareCopyDestinationCtx(ctx, sources, destination); err != nil {
		return nil, 0, 0, 0, err
	}
	ti, td, tb := SummarizePlan(p)
	return p, ti, td, tb, nil
}

// CopyPlanTotals returns the file count and byte sum for a copy plan after the same
// destination validation as ExecuteCopy. Used to populate job totals before transfer.
func CopyPlanTotals(sources []pathloc.Path, destination pathloc.Path) (totalFiles int, totalBytes int64, err error) {
	tf, _, tb, err := CopyPlanTotalsDetailed(sources, destination)
	return tf, tb, err
}

// CopyPlanTotalsDetailed returns item count, directory count, and byte sum for a copy plan.
func CopyPlanTotalsDetailed(sources []pathloc.Path, destination pathloc.Path) (totalItems, totalDirs int, totalBytes int64, err error) {
	_, ti, td, tb, err := BuildCopyPlanWithTotalsCtx(context.Background(), sources, destination, PlanBuildOptions{})
	return ti, td, tb, err
}

func prepareCopyDestinationCtx(ctx context.Context, sources []pathloc.Path, destination pathloc.Path) error {
	isDir, err := destinationIsDir(ctx, destination)
	if err != nil {
		return fmt.Errorf("stat destination %q: %w", destination, err)
	}
	if isDir {
		return nil
	}
	if len(sources) == 1 {
		srcEnt, err := statEntry(ctx, sources[0])
		if err != nil {
			return fmt.Errorf("stat source %q: %w", sources[0], err)
		}
		if srcEnt.Type == fsbackend.EntryDirectory {
			be, err := backendFor(destination)
			if err != nil {
				return err
			}
			if err := be.Mkdir(ctx, destination, 0o755); err != nil {
				return fmt.Errorf("create destination dir %q: %w", destination, err)
			}
			return nil
		}
		return nil
	}
	return fmt.Errorf("destination directory %q does not exist", destination)
}

// SummarizePlan returns plan item count, directory count, and regular-file byte sum.
func SummarizePlan(plan []PlanItem) (totalItems, totalDirs int, totalBytes int64) {
	for _, item := range plan {
		totalItems++
		if item.IsDir {
			totalDirs++
		}
		if !item.IsDir && !item.IsSymlink {
			totalBytes += item.FileSize
		}
	}
	return totalItems, totalDirs, totalBytes
}

// SummarizePlanForSource returns plan item and regular-file byte counts for entries under root.
func SummarizePlanForSource(plan []PlanItem, root pathloc.Path) (items int, bytes int64) {
	for _, item := range plan {
		if !pathloc.EqualOrUnder(root, item.Src) {
			continue
		}
		items++
		if !item.IsDir && !item.IsSymlink {
			bytes += item.FileSize
		}
	}
	return items, bytes
}

// ExecuteCopy copies a set of source paths to a destination.
// It handles regular files, directories, and symlinks.
// Returns (doneFiles, doneBytes, error).
// diskWait is optional; when set, regular file copies wait until there is enough free space on the destination volume.
func ExecuteCopy(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	doneFiles, doneBytes, _, err := executeCopyWithPlan(ctx, nil, sources, destination, opts, throttle, progress, resolver, diskWait)
	return doneFiles, doneBytes, err
}

// ExecuteCopyUsingPlan runs the copy loop using a plan from BuildCopyPlanWithTotals for the same sources and destination
// (prepareCopyDestination and BuildPlan are skipped). Caller must not mutate the plan slice during the call.
func ExecuteCopyUsingPlan(ctx context.Context, plan []PlanItem, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	if plan == nil {
		return 0, 0, fmt.Errorf("ExecuteCopyUsingPlan: plan is nil")
	}
	doneFiles, doneBytes, _, err := executeCopyWithPlan(ctx, plan, sources, destination, opts, throttle, progress, resolver, diskWait)
	return doneFiles, doneBytes, err
}

// copyRunState carries the mutable tallies and throttled-progress state shared
// across the per-item copy helpers invoked from executeCopyWithPlan's loop.
type copyRunState struct {
	doneFiles       int
	doneBytes       int64
	bytesSinceEmit  int64
	lastEmit        time.Time
	lastMetaEmit    time.Time // mkdir/symlink progress (no byte deltas): throttle by MinInterval only
	transferredOut  []pathloc.Path
	deferredDirMeta []PlanItem
	deferredSync    []string

	th       ProgressEmitThrottle
	progress ProgressCallback
}

func (s *copyRunState) recordTransferred(src pathloc.Path) {
	s.transferredOut = append(s.transferredOut, src)
}

func (s *copyRunState) emitProgress(srcPath, dstPath string, force bool) {
	if s.progress == nil {
		return
	}
	now := time.Now()
	sinceEmit := time.Duration(0)
	if !s.lastEmit.IsZero() {
		sinceEmit = now.Sub(s.lastEmit)
	}
	if force || s.bytesSinceEmit >= s.th.MinBytes || (s.bytesSinceEmit > 0 && !s.lastEmit.IsZero() && sinceEmit >= s.th.MinInterval) {
		s.progress(srcPath, dstPath, s.doneFiles, s.doneBytes)
		s.bytesSinceEmit = 0
		s.lastEmit = now
	}
}

func (s *copyRunState) emitMetaProgress(srcPath, dstPath string) {
	if s.progress == nil {
		return
	}
	now := time.Now()
	if !s.lastMetaEmit.IsZero() && now.Sub(s.lastMetaEmit) < s.th.MinInterval {
		return
	}
	s.lastMetaEmit = now
	s.progress(srcPath, dstPath, s.doneFiles, s.doneBytes)
}

func copyDirItem(ctx context.Context, item PlanItem, opts Options, state *copyRunState) error {
	srcStr := item.Src.String()
	dstStr := item.Dst.String()
	if _, err := statEntry(ctx, item.Dst); isNotExist(err) {
		be, err := backendFor(item.Dst)
		if err != nil {
			return err
		}
		if err := be.Mkdir(ctx, item.Dst, mkdirModeForItem(item, opts)); err != nil {
			return fmt.Errorf("create directory %q: %w", dstStr, err)
		}
	} else if err != nil {
		return fmt.Errorf("stat directory %q: %w", dstStr, PathErrorReason(err))
	}
	if opts.PreservePermissions || opts.PreserveTimestamps {
		state.deferredDirMeta = append(state.deferredDirMeta, item)
	}
	state.doneFiles++
	state.emitMetaProgress(srcStr, dstStr)
	return nil
}

func copySymlinkItem(ctx context.Context, item PlanItem, resolver ConflictResolver, state *copyRunState) error {
	srcStr := item.Src.String()
	dstStr := item.Dst.String()
	var copied bool
	if useLocalFastPath(item.Src, item.Dst) {
		var err error
		copied, err = copySymlinkWithConflict(srcStr, dstStr, resolver)
		if err != nil {
			return err
		}
	} else {
		var err error
		copied, err = copySymlinkTransfer(ctx, item.Src, item.Dst, resolver)
		if err != nil {
			return err
		}
	}
	if !copied {
		return nil
	}
	state.recordTransferred(item.Src)
	state.doneFiles++
	state.emitMetaProgress(srcStr, dstStr)
	return nil
}

func copyRegularItem(ctx context.Context, item PlanItem, opts Options, resolver ConflictResolver, diskWait DiskWaitFunc, destination pathloc.Path, copyBuf []byte, state *copyRunState) error {
	srcStr := item.Src.String()
	dstStr := item.Dst.String()

	if diskWait != nil && destination.Scheme() == pathloc.SchemeFile &&
		(opts.DiskSpaceCheckMinFileBytes <= 0 || item.FileSize >= opts.DiskSpaceCheckMinFileBytes) {
		if err := EnsureDiskSpace(diskWait, destination, item.FileSize, item.Src); err != nil {
			return err
		}
	}

	var copied bool
	var err error
	if useLocalFastPath(item.Src, item.Dst) {
		copied, err = copyFileWithConflict(ctx, srcStr, dstStr, opts, resolver, copyBuf, func(delta int64) {
			state.doneBytes += delta
			state.bytesSinceEmit += delta
			state.emitProgress(srcStr, dstStr, false)
		})
	} else {
		copied, err = copyFileTransfer(ctx, item.Src, item.Dst, opts, resolver, copyBuf, func(delta int64) {
			state.doneBytes += delta
			state.bytesSinceEmit += delta
			state.emitProgress(srcStr, dstStr, false)
		})
	}
	if err != nil {
		return err
	}
	if !copied {
		return nil
	}
	state.recordTransferred(item.Src)

	if opts.SyncFileDeferred(item.FileSize) && item.Dst.Scheme() == pathloc.SchemeFile {
		if host, err := item.Dst.FilePath(); err == nil {
			state.deferredSync = append(state.deferredSync, host)
		}
	}

	state.doneFiles++
	state.emitProgress(srcStr, dstStr, true)
	return nil
}

func applyDeferredDirMeta(ctx context.Context, opts Options, state *copyRunState) error {
	for _, item := range state.deferredDirMeta {
		if err := applyItemMetadata(ctx, item, opts); err != nil {
			return err
		}
	}
	return nil
}

func runDeferredSyncs(state *copyRunState) error {
	for _, path := range state.deferredSync {
		if err := syncLocalPath(path); err != nil {
			return fmt.Errorf("sync destination %q: %w", path, err)
		}
	}
	return nil
}

func executeCopyWithPlan(ctx context.Context, planOptional []PlanItem, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, []pathloc.Path, error) {
	var plan []PlanItem
	var err error
	if planOptional != nil {
		plan = planOptional
	} else {
		plan, err = BuildPlanCtx(ctx, sources, destination, true, PlanBuildOptions{FlatDestNames: opts.FlatDestNames})
		if err != nil {
			return 0, 0, nil, fmt.Errorf("build copy plan: %w", err)
		}
		if err := prepareCopyDestinationCtx(ctx, sources, destination); err != nil {
			return 0, 0, nil, err
		}
	}

	state := &copyRunState{th: effectiveProgressThrottle(throttle), progress: progress}
	copyBuf := make([]byte, BufferSize(opts.CopyBufferKiB))

	for _, item := range plan {
		if err := ctx.Err(); err != nil {
			return state.doneFiles, state.doneBytes, state.transferredOut, err
		}
		if item.IsDir && !item.IsSymlink {
			if err := copyDirItem(ctx, item, opts, state); err != nil {
				return state.doneFiles, state.doneBytes, state.transferredOut, err
			}
			continue
		}

		if item.IsSymlink {
			if err := copySymlinkItem(ctx, item, resolver, state); err != nil {
				return state.doneFiles, state.doneBytes, state.transferredOut, err
			}
			continue
		}

		if err := copyRegularItem(ctx, item, opts, resolver, diskWait, destination, copyBuf, state); err != nil {
			return state.doneFiles, state.doneBytes, state.transferredOut, err
		}
	}

	if err := applyDeferredDirMeta(ctx, opts, state); err != nil {
		return state.doneFiles, state.doneBytes, state.transferredOut, err
	}

	if err := runDeferredSyncs(state); err != nil {
		return state.doneFiles, state.doneBytes, state.transferredOut, err
	}

	return state.doneFiles, state.doneBytes, state.transferredOut, nil
}

func copyFileWithConflict(ctx context.Context, src, dst string, opts Options, resolver ConflictResolver, buf []byte, onWritten func(int64)) (copied bool, err error) {
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return false, fmt.Errorf("create parent directory %q: %w", parent, err)
	}

	if _, err := os.Stat(dst); err == nil {
		if resolver == nil {
			return false, fmt.Errorf("destination %q already exists and no conflict resolver configured", dst)
		}
		facts, err := StatFileConflictFacts(src, dst)
		if err != nil {
			return false, fmt.Errorf("conflict stat %q %q: %w", src, dst, err)
		}
		overwrite, err := resolver(src, dst, facts)
		if err != nil {
			return false, err
		}
		if !overwrite {
			return false, nil
		}
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat destination %q: %w", dst, err)
	}

	err = localfs.CopyFile(ctx, src, dst, BufferSize(opts.CopyBufferKiB), opts.PreservePermissions, opts.PreserveTimestamps, false, opts.CowFileCloning, opts.LocalCopyFileOpts(buf), onWritten)
	if err != nil {
		return false, err
	}
	return true, nil
}

func copySymlinkWithConflict(src, dst string, resolver ConflictResolver) (copied bool, err error) {
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return false, fmt.Errorf("create parent directory %q: %w", parent, err)
	}

	if _, err := os.Lstat(dst); err == nil {
		if resolver == nil {
			return false, fmt.Errorf("destination %q already exists and no conflict resolver configured", dst)
		}
		facts, err := StatFileConflictFacts(src, dst)
		if err != nil {
			return false, fmt.Errorf("conflict stat %q %q: %w", src, dst, err)
		}
		overwrite, err := resolver(src, dst, facts)
		if err != nil {
			return false, err
		}
		if !overwrite {
			return false, nil
		}
		if err := os.Remove(dst); err != nil {
			return false, fmt.Errorf("remove existing %q: %w", dst, err)
		}
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat destination %q: %w", dst, err)
	}

	if err := CopySymlink(src, dst, nil); err != nil {
		return false, err
	}
	return true, nil
}
