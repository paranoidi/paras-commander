package ops

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
	// DereferenceSymlinks mirrors Options.DereferenceSymlinks for plan building: symlinks
	// under a walked source are resolved and classified as their target (file or directory)
	// instead of as a symlink. See planLocalSource / localfs.WalkDirRecursiveDeref.
	DereferenceSymlinks bool
	// OnWarning reports a non-fatal per-item issue during plan building (e.g. a symlink that
	// fell back to being relinked instead of dereferenced). Optional; nil-safe.
	OnWarning func(string)
	// OnItemError reports a source item the walk cannot plan (e.g. a socket or other unsupported
	// file type) and skips it instead of aborting the walk. Nil keeps the abort behavior.
	OnItemError func(item PlanItem, err error)
}

// ConflictResolver is called when a destination path already exists.
// It receives source and destination paths plus file metadata and returns what to do with the
// destination (overwrite, skip, identical, rename, delete source), and an error to abort.
type ConflictResolver func(ctx context.Context, src, dest string, facts FileConflictFacts) (ConflictResolution, error)

// copyResult is the outcome of copying one file or symlink.
type copyResult int

const (
	copySkipped       copyResult = iota // resolver declined; nothing written
	copyDone                            // written
	copyIdentical                       // destination already identical; nothing written
	copySourceDeleted                   // move only: destination kept, source to be removed
)

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

// validateBatchDestinationCtx enforces prepareCopyDestinationCtx's "destination directory must
// already exist for a multi-source copy" rule without its single-directory-source auto-mkdir.
// BuildPlanStreamCtx uses this instead of prepareCopyDestinationCtx: mkdir'ing a new destination
// name before the streamed walk resolves it would trick resolveDestinationNamedCtx into treating
// dest as an existing container (see BuildCopyPlanWithTotalsCtx's ordering comment) — instead,
// the top-level directory PlanItem the walk produces first reaches copyDirItem, which already
// mkdir's a missing destination lazily, same as prepareCopyDestinationCtx would have.
func validateBatchDestinationCtx(ctx context.Context, sources []pathloc.Path, destination pathloc.Path) error {
	isDir, err := destinationIsDir(ctx, destination)
	if err != nil {
		return fmt.Errorf("stat destination %q: %w", destination, err)
	}
	if isDir || len(sources) == 1 {
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

// ExecuteCopy copies a set of source paths to a destination.
// It handles regular files, directories, and symlinks.
// Returns (doneFiles, doneBytes, error).
// diskWait is optional; when set, regular file copies wait until there is enough free space on the destination volume.
func ExecuteCopy(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	doneFiles, doneBytes, _, err := transferRun{
		ctx: ctx, sources: sources, destination: destination, opts: opts,
		throttle: throttle, progress: progress, resolver: resolver, diskWait: diskWait,
	}.executeCopy()
	return doneFiles, doneBytes, err
}

// ExecuteCopyUsingPlan runs the copy loop using a plan from BuildCopyPlanWithTotals for the same sources and destination
// (prepareCopyDestination and BuildPlan are skipped). Caller must not mutate the plan slice during the call.
func ExecuteCopyUsingPlan(ctx context.Context, plan []PlanItem, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	if plan == nil {
		return 0, 0, fmt.Errorf("ExecuteCopyUsingPlan: plan is nil")
	}
	doneFiles, doneBytes, _, err := transferRun{
		ctx: ctx, sources: sources, destination: destination, opts: opts,
		throttle: throttle, progress: progress, resolver: resolver, diskWait: diskWait,
		planOptional: plan,
	}.executeCopy()
	return doneFiles, doneBytes, err
}

// ExecuteCopyUsingPlanChan runs the copy loop consuming a streamed plan channel (from
// BuildPlanStreamCtx) instead of a pre-built slice, so transfer can start before the producer
// finishes enumerating the source tree. planErr, when non-nil, is called once planCh is observed
// closed to retrieve the producer's terminal error (nil on a clean end); it must be safe to call
// concurrently with the producer still running (e.g. a mutex-guarded closure), since planCh may
// close for reasons unrelated to the producer having fully stored its result yet. That error
// becomes this call's returned error, unless an earlier item-level error already aborted the loop.
func ExecuteCopyUsingPlanChan(ctx context.Context, planCh <-chan PlanItem, planErr func() error, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	doneFiles, doneBytes, _, err := executeCopyIter(ctx, planIterChan(ctx, planCh, planErr), destination, opts, throttle, progress, resolver, diskWait)
	return doneFiles, doneBytes, err
}

// planIter yields one plan item per call until exhausted. ok is false once iteration ends;
// err carries the terminal error, if any (nil on a clean end). Shared by the slice-backed
// (ExecuteCopyUsingPlan) and channel-backed (*Chan) executors so the
// per-item copy loop in executeCopyIter is written once.
type planIter func() (item PlanItem, ok bool, err error)

// planIterSlice iterates a pre-built plan slice — no behavior change from ranging over it directly.
func planIterSlice(plan []PlanItem) planIter {
	i := 0
	return func() (PlanItem, bool, error) {
		if i >= len(plan) {
			return PlanItem{}, false, nil
		}
		it := plan[i]
		i++
		return it, true, nil
	}
}

// planIterChan iterates a streamed plan channel, selecting on ctx so cancellation interrupts a
// blocked receive. See ExecuteCopyUsingPlanChan for the planErr contract.
func planIterChan(ctx context.Context, planCh <-chan PlanItem, planErr func() error) planIter {
	return func() (PlanItem, bool, error) {
		select {
		case it, ok := <-planCh:
			if !ok {
				if planErr != nil {
					return PlanItem{}, false, planErr()
				}
				return PlanItem{}, false, nil
			}
			return it, true, nil
		case <-ctx.Done():
			return PlanItem{}, false, ctx.Err()
		}
	}
}

// copyRunState carries the mutable tallies and throttled-progress state shared
// across the per-item copy helpers invoked from executeCopyWithPlan's loop.
type copyRunState struct {
	doneFiles       int
	processed       int // non-directory plan items gone through (Options.OnItemsProcessed)
	doneBytes       int64
	bytesSinceEmit  int64
	lastEmit        time.Time
	lastMetaEmit    time.Time // mkdir/symlink progress (no byte deltas): throttle by MinInterval only
	transferredOut  []pathloc.Path
	deferredDirMeta []PlanItem
	deferredSync    []string
	skippedDirs     []pathloc.Path    // source dirs whose destination collided with a non-directory and was skipped
	renamedDirs     [][2]pathloc.Path // {destination, keep-both sibling} for directories written under a new name

	th       ProgressEmitThrottle
	progress ProgressCallback
}

// countProcessed counts item toward Options.OnItemsProcessed unless it is a directory (IsDir,
// the same rule the counting walk tallies TotalDirs by).
func (s *copyRunState) countProcessed(item PlanItem) {
	if !item.IsDir {
		s.processed++
	}
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

// skipsItem reports whether item lies under a skipped directory; otherwise it re-roots item.Dst
// under a directory that was kept under a new name. Plans list a directory before its children.
func (s *copyRunState) skipsItem(item *PlanItem) bool {
	for _, d := range s.skippedDirs {
		if item.Src.HasPrefix(d) {
			return true
		}
	}
	for _, r := range s.renamedDirs {
		if item.Dst.HasPrefix(r[0]) {
			if np, err := pathloc.Parse(r[1].String() + strings.TrimPrefix(item.Dst.String(), r[0].String())); err == nil {
				item.Dst = np
			}
			break
		}
	}
	return false
}

func copyDirItem(ctx context.Context, item PlanItem, opts Options, resolver ConflictResolver, state *copyRunState) error {
	de, statErr := statEntry(ctx, item.Dst)
	if statErr == nil && de.Type != fsbackend.EntryDirectory {
		// A file or symlink (even to a directory) sits where the directory goes: never merge
		// through it, never replace it; only skip or keep both.
		res, newDst, err := resolveDestConflict(ctx, item.Src, item.Dst, resolver)
		if err != nil {
			return err
		}
		if res != copyDone {
			state.skippedDirs = append(state.skippedDirs, item.Src)
			opts.itemDone(item, ItemSkipped, ReasonExists)
			return nil
		}
		state.renamedDirs = append(state.renamedDirs, [2]pathloc.Path{item.Dst, newDst})
		item.Dst, statErr = newDst, fs.ErrNotExist
	}
	srcStr := item.Src.String()
	dstStr := item.Dst.String()
	if isNotExist(statErr) {
		be, err := backendFor(item.Dst)
		if err != nil {
			return err
		}
		if err := be.Mkdir(ctx, item.Dst, mkdirModeForItem(item, opts)); err != nil {
			return fmt.Errorf("create directory %q: %w", dstStr, err)
		}
		opts.journalCreated(ctx, item.Dst)
	} else if statErr != nil {
		return fmt.Errorf("stat directory %q: %w", dstStr, PathErrorReason(statErr))
	}
	if opts.PreservePermissions || opts.PreserveTimestamps {
		state.deferredDirMeta = append(state.deferredDirMeta, item)
	}
	state.doneFiles++
	state.emitMetaProgress(srcStr, dstStr)
	return nil
}

func copySymlinkItem(ctx context.Context, item PlanItem, opts Options, resolver ConflictResolver, state *copyRunState) error {
	srcStr := item.Src.String()
	dstStr := item.Dst.String()
	var res copyResult
	var err error
	if useLocalFastPath(item.Src, item.Dst) {
		res, err = copySymlinkWithConflict(ctx, srcStr, dstStr, opts, resolver)
	} else {
		res, err = copySymlinkTransfer(ctx, item.Src, item.Dst, opts, resolver)
	}
	if err != nil {
		return err
	}
	if !reportCopyResult(opts, item, res) {
		return nil
	}
	state.recordTransferred(item.Src)
	if res == copySourceDeleted {
		return nil
	}
	state.doneFiles++
	state.emitMetaProgress(srcStr, dstStr)
	return nil
}

// reportCopyResult reports a file or symlink copy outcome through opts.OnItemDone and returns
// false when nothing was transferred (the resolver skipped it). Delete source counts as
// transferred so a move's copy phase removes the source.
func reportCopyResult(opts Options, item PlanItem, res copyResult) bool {
	switch res {
	case copySkipped:
		opts.itemDone(item, ItemSkipped, ReasonExists)
		return false
	case copyIdentical:
		opts.itemDone(item, ItemSkipped, ReasonIdentical)
		return false
	case copySourceDeleted:
		opts.itemDone(item, ItemSourceDeleted, ReasonSourceDeleted)
	default:
		opts.itemDone(item, ItemDone, "")
	}
	return true
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

	var res copyResult
	var err error
	if useLocalFastPath(item.Src, item.Dst) {
		res, err = copyFileWithConflict(ctx, srcStr, dstStr, opts, resolver, copyBuf, func(delta int64) {
			// ponytail: throttles after each buffer write, so kernel fast-path copies
			// (reflink/copy_file_range, which report their whole size in one onWritten
			// call) get throttled as a post-hoc approximation rather than smoothly.
			if opts.RateLimit != nil {
				_ = opts.RateLimit(ctx, int(delta))
			}
			state.doneBytes += delta
			state.bytesSinceEmit += delta
			state.emitProgress(srcStr, dstStr, false)
		})
	} else {
		res, err = copyFileTransfer(ctx, item.Src, item.Dst, opts, resolver, copyBuf, func(delta int64) {
			if opts.RateLimit != nil {
				_ = opts.RateLimit(ctx, int(delta))
			}
			state.doneBytes += delta
			state.bytesSinceEmit += delta
			state.emitProgress(srcStr, dstStr, false)
		})
	}
	if err != nil {
		return err
	}
	if !reportCopyResult(opts, item, res) {
		return nil
	}
	state.recordTransferred(item.Src)
	if res == copySourceDeleted {
		return nil
	}

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
		plan, err = BuildPlanCtx(ctx, sources, destination, true, PlanBuildOptions{FlatDestNames: opts.FlatDestNames, DereferenceSymlinks: opts.DereferenceSymlinks})
		if err != nil {
			return 0, 0, nil, fmt.Errorf("build copy plan: %w", err)
		}
		if err := prepareCopyDestinationCtx(ctx, sources, destination); err != nil {
			return 0, 0, nil, err
		}
	}
	return executeCopyIter(ctx, planIterSlice(plan), destination, opts, throttle, progress, resolver, diskWait)
}

// executeCopyIter runs the per-item copy loop shared by every plan source (pre-built slice or
// streamed channel): it drives iter to exhaustion, dispatching each item to the matching
// copyDirItem/copySymlinkItem/copyRegularItem helper, then applies deferred dir metadata and
// syncs once the plan is exhausted.
func executeCopyIter(ctx context.Context, iter planIter, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, []pathloc.Path, error) {
	state := &copyRunState{th: effectiveProgressThrottle(throttle), progress: progress}
	copyBuf := make([]byte, BufferSize(opts.CopyBufferKiB))
	if opts.OnItemsProcessed != nil {
		defer func() { opts.OnItemsProcessed(state.processed) }()
	}

	for {
		if err := ctx.Err(); err != nil {
			return state.doneFiles, state.doneBytes, state.transferredOut, err
		}
		item, ok, err := iter()
		if !ok {
			if err != nil {
				return state.doneFiles, state.doneBytes, state.transferredOut, err
			}
			break
		}

		opts.enterSource(item.Source)
		if state.skipsItem(&item) {
			state.countProcessed(item)
			continue
		}

		if item.IsDir && !item.IsSymlink {
			if err := copyDirItem(ctx, item, opts, resolver, state); err != nil {
				return state.doneFiles, state.doneBytes, state.transferredOut, err
			}
			continue
		}

		if item.IsSymlink {
			if err := copySymlinkItem(ctx, item, opts, resolver, state); err != nil {
				return state.doneFiles, state.doneBytes, state.transferredOut, err
			}
			state.countProcessed(item)
			continue
		}

		if err := copyRegularItem(ctx, item, opts, resolver, diskWait, destination, copyBuf, state); err != nil && !skipSourceItemError(ctx, opts, item, err) {
			return state.doneFiles, state.doneBytes, state.transferredOut, err
		}
		state.countProcessed(item)
	}

	if err := applyDeferredDirMeta(ctx, opts, state); err != nil {
		return state.doneFiles, state.doneBytes, state.transferredOut, err
	}

	if err := runDeferredSyncs(state); err != nil {
		return state.doneFiles, state.doneBytes, state.transferredOut, err
	}

	return state.doneFiles, state.doneBytes, state.transferredOut, nil
}

// skipSourceItemError reports err through opts.OnItemError and returns true when the copy may
// carry on without item: a source-side failure (localfs.IsSourceError) with the callback set and
// ctx still live. Destination, conflict and cancel errors are never skipped.
func skipSourceItemError(ctx context.Context, opts Options, item PlanItem, err error) bool {
	if opts.OnItemError == nil || ctx.Err() != nil || !localfs.IsSourceError(err) {
		return false
	}
	opts.OnItemError(item, err)
	return true
}

func copyFileWithConflict(ctx context.Context, src, dst string, opts Options, resolver ConflictResolver, buf []byte, onWritten func(int64)) (copyResult, error) {
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return copySkipped, fmt.Errorf("create parent directory %q: %w", parent, err)
	}

	if _, err := os.Stat(dst); err == nil {
		facts, ferr := StatFileConflictFacts(src, dst)
		if ferr != nil {
			return copySkipped, fmt.Errorf("conflict stat %q %q: %w", src, dst, ferr)
		}
		newDst, res, done, perr := resolveLocalConflict(ctx, src, dst, resolver, facts)
		if perr != nil || done {
			return res, perr
		}
		return copyLocalFile(ctx, src, newDst, opts, buf, onWritten)
	} else if !os.IsNotExist(err) {
		return copySkipped, fmt.Errorf("stat destination %q: %w", dst, err)
	}
	return copyLocalFile(ctx, src, dst, opts, buf, onWritten)
}

func copyLocalFile(ctx context.Context, src, dst string, opts Options, buf []byte, onWritten func(int64)) (copyResult, error) {
	err := localfs.CopyFile(ctx, src, dst, BufferSize(opts.CopyBufferKiB), opts.PreservePermissions, opts.PreserveTimestamps, false, opts.CowFileCloning, opts.LocalCopyFileOpts(buf), onWritten)
	if err != nil {
		return copySkipped, err
	}
	journalLocalCreated(ctx, opts, dst)
	return copyDone, nil
}

// journalLocalCreated is Options.journalCreated for a local path string.
func journalLocalCreated(ctx context.Context, opts Options, dst string) {
	if opts.OnJournal == nil {
		return
	}
	if loc, err := pathloc.Parse(dst); err == nil {
		opts.journalCreated(ctx, loc)
	}
}

// resolveLocalConflict adapts resolveConflict to local string paths. done is true when the
// caller must stop and return res (skip, identical or delete source); otherwise write to the
// returned dst.
func resolveLocalConflict(ctx context.Context, src, dst string, resolver ConflictResolver, facts FileConflictFacts) (newDst string, res copyResult, done bool, err error) {
	sp, err := pathloc.Parse(src)
	if err != nil {
		return dst, copySkipped, true, err
	}
	dp, err := pathloc.Parse(dst)
	if err != nil {
		return dst, copySkipped, true, err
	}
	action, np, err := resolveConflict(ctx, sp, dp, resolver, facts)
	if err != nil {
		return dst, copySkipped, true, err
	}
	switch action {
	case ActionOverwrite:
		return dst, copyDone, false, nil
	case ActionRename:
		return np.String(), copyDone, false, nil
	case ActionIdentical:
		return dst, copyIdentical, true, nil
	case ActionDeleteSource:
		return dst, copySourceDeleted, true, nil
	}
	return dst, copySkipped, true, nil
}

func copySymlinkWithConflict(ctx context.Context, src, dst string, opts Options, resolver ConflictResolver) (copyResult, error) {
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return copySkipped, fmt.Errorf("create parent directory %q: %w", parent, err)
	}

	if _, err := os.Lstat(dst); err == nil {
		facts, ferr := StatFileConflictFacts(src, dst)
		if ferr != nil {
			return copySkipped, fmt.Errorf("conflict stat %q %q: %w", src, dst, ferr)
		}
		newDst, res, done, perr := resolveLocalConflict(ctx, src, dst, resolver, facts)
		if perr != nil || done {
			return res, perr
		}
		if newDst == dst {
			if err := os.Remove(dst); err != nil {
				return copySkipped, fmt.Errorf("remove existing %q: %w", dst, err)
			}
		}
		dst = newDst
	} else if !os.IsNotExist(err) {
		return copySkipped, fmt.Errorf("stat destination %q: %w", dst, err)
	}

	if err := CopySymlink(src, dst, nil); err != nil {
		return copySkipped, err
	}
	journalLocalCreated(ctx, opts, dst)
	return copyDone, nil
}
