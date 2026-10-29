package ops

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// resolveMoveDestination is ResolveDestinationNamed using the job context, so a canceled dest
// Stat unwinds the rename phase.
func resolveMoveDestination(ctx context.Context, dest pathloc.Path, name string) (pathloc.Path, error) {
	if err := ctx.Err(); err != nil {
		return pathloc.Path{}, err
	}
	isDir, err := destinationIsDir(ctx, dest)
	if err != nil {
		if ctx.Err() != nil {
			return pathloc.Path{}, ctx.Err()
		}
		return dest, nil
	}
	if isDir {
		child, err := dest.Join(name)
		if err != nil {
			return dest, nil
		}
		return child, nil
	}
	return dest, nil
}

// RenameFastPathCtx is RenameFastPath with a caller context for remote backends.
// Local os.Rename is not context-aware.
func RenameFastPathCtx(ctx context.Context, src, dest pathloc.Path) (ok bool, err error) {
	if src.Scheme() != dest.Scheme() {
		return false, nil
	}
	if src.IsRemote() {
		if !sameSFTPHost(src, dest) {
			return false, nil
		}
		be, err := backendFor(src)
		if err != nil {
			return false, err
		}
		if renameErr := be.Rename(ctx, src, dest); renameErr != nil {
			if errors.Is(renameErr, context.Canceled) || errors.Is(renameErr, context.DeadlineExceeded) {
				return false, renameErr
			}
			return false, nil
		}
		return true, nil
	}
	return RenameFastPath(ctx, src, dest)
}

func stageExistingDest(ctx context.Context, dst pathloc.Path) (pathloc.Path, error) {
	staged, err := pickMoveStashSibling(ctx, dst)
	if err != nil {
		return pathloc.Path{}, err
	}
	ok, err := RenameFastPathCtx(ctx, dst, staged)
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
	ok, err := RenameFastPathCtx(ctx, staged, dst)
	if err != nil {
		return fmt.Errorf("restore staged %q -> %q: %w", staged, dst, err)
	}
	if !ok {
		return fmt.Errorf("restore staged %q -> %q: cannot rename", staged, dst)
	}
	return nil
}

// renameOutcome is renameSourceForMove's result for one source.
type renameOutcome struct {
	renamed      bool // moved by rename (including the unique sibling name for keep-both)
	fallbackCopy bool // cross-device (or non-fast) rename: copy+delete for the batch
	// keepBoth: the fallback copy must write the source under a free sibling name instead of
	// asking the resolver again.
	keepBoth bool
	staged   string // parked original destination after an overwrite; empty otherwise
	// sourceDeleted: the destination was kept and the source was just removed.
	sourceDeleted bool
	// identical: the destination already holds the same content; nothing moved, source kept.
	identical bool
	// final is the path the source ended up at when that is not the requested dst (keep-both).
	final pathloc.Path
}

// renameSourceForMove handles conflict resolution then RenameFastPath for one source. Delete
// source removes the source and keeps the destination. dstExists is the caller's stat of dst.
func renameSourceForMove(ctx context.Context, src, dst pathloc.Path, dstExists bool, resolver ConflictResolver) (renameOutcome, error) {
	if err := ctx.Err(); err != nil {
		return renameOutcome{}, err
	}
	if !dstExists {
		return renameFastPathOrFallback(ctx, src, dst)
	}
	if resolver == nil {
		return renameOutcome{}, fmt.Errorf("destination %q already exists and no conflict resolver configured", dst)
	}
	facts, err := statConflictFacts(ctx, src, dst)
	if err != nil {
		return renameOutcome{}, fmt.Errorf("conflict stat %q %q: %w", src, dst, err)
	}
	action, newDst, err := resolveConflict(ctx, src, dst, resolver, facts)
	if err != nil {
		return renameOutcome{}, err
	}
	switch action {
	case ActionDeleteSource:
		if err := removePathRecursive(ctx, src); err != nil {
			return renameOutcome{}, fmt.Errorf("remove source %q: %w", src, err)
		}
		return renameOutcome{sourceDeleted: true}, nil
	case ActionIdentical:
		return renameOutcome{identical: true}, nil // skipped
	case ActionRename:
		out, err := renameFastPathOrFallback(ctx, src, newDst)
		out.keepBoth = out.fallbackCopy
		out.final = newDst
		return out, err
	case ActionOverwrite:
	default:
		return renameOutcome{}, nil // skipped
	}
	if !dst.IsRemote() {
		// rename(2) atomically replaces a non-directory target; the mismatch gate in
		// resolveConflict guarantees dst is not a directory here.
		return renameFastPathOrFallback(ctx, src, dst)
	}
	// SFTP v3 rename fails when the target exists: park the original, rename in, then drop it.
	stagedLoc, stageErr := stageExistingDest(ctx, dst)
	if stageErr != nil {
		return renameOutcome{}, stageErr
	}
	out, err := renameFastPathOrFallback(ctx, src, dst)
	if err != nil || out.fallbackCopy || !out.renamed {
		if restoreErr := restoreStagedDest(ctx, dst, stagedLoc); restoreErr != nil {
			if err != nil {
				return renameOutcome{}, fmt.Errorf("%w (restore staged dest: %v)", err, restoreErr)
			}
			return renameOutcome{}, restoreErr
		}
		return renameOutcome{fallbackCopy: out.fallbackCopy}, err
	}
	out.staged = stagedLoc.String()
	return out, nil
}

func renameFastPathOrFallback(ctx context.Context, src, dst pathloc.Path) (renameOutcome, error) {
	ok, err := RenameFastPathCtx(ctx, src, dst)
	if err != nil {
		return renameOutcome{}, err
	}
	if !ok {
		return renameOutcome{fallbackCopy: true}, nil
	}
	return renameOutcome{renamed: true}, nil
}

// ExecuteMove moves each source to destination in the style of mc: try the O(1) rename first and
// only plan, size and copy+delete a source whose rename cannot be used (cross device or cross
// host). It runs in two phases. The rename phase renames every source it can (merging directories
// onto existing ones) and sets aside what needs copying; opts.OnCopyPhase then receives the
// settled units and a counting walk over the set-aside work, and the copy phase copies it. Earlier
// renames are never rolled back when a later source fails or needs the copy fallback. doneFiles
// counts top-level sources moved by rename plus the items copied for fallback sources.
func ExecuteMove(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	var nameRoot pathloc.Path
	if !opts.FlatDestNames {
		nameRoot = TransferNameRoot(sources)
	}
	var pending []moveTask
	r := moveRun{opts: opts, throttle: throttle, progress: progress, resolver: resolver, diskWait: diskWait, pending: &pending}
	var doneFiles int
	var doneBytes int64
	hints := opts.SourceSizeHints
	if len(hints) != len(sources) {
		hints = nil
	}
	for i, src := range sources {
		if err := ctx.Err(); err != nil {
			return doneFiles, doneBytes, err
		}
		r.opts.enterSource(int32(i))
		name := TransferDestName(src, nameRoot)
		dst, err := resolveMoveDestination(ctx, destination, name)
		if err != nil {
			return doneFiles, doneBytes, err
		}
		// A nested child name or a file-target destination (dst is destination itself) may
		// name a directory that does not exist yet.
		if strings.ContainsAny(name, `/\`) || dst.Equal(destination) {
			if err := ensureParentDirs(ctx, dst); err != nil {
				return doneFiles, doneBytes, fmt.Errorf("create parent for %q: %w", dst, err)
			}
		}
		setAside := len(pending)
		moved, err := r.collect(ctx, src, dst, destination, true)
		if err != nil {
			return doneFiles, doneBytes, err
		}
		if len(pending) > setAside {
			// The copy phase moves the rest: the source is queued again, not finished.
			r.opts.deferSource(int32(i))
		}
		if !moved {
			continue
		}
		doneFiles++
		// A rename moves no bytes through the copier: credit the source's estimate. Sources
		// left for the copy phase are credited by the copier instead.
		if hints != nil && hints[i] > 0 {
			doneBytes += hints[i]
		}
		if r.progress != nil {
			r.progress(src.String(), dst.String(), doneFiles, doneBytes)
		}
	}
	if r.opts.OnCopyPhase != nil {
		phase := MoveCopyPhase{Items: doneFiles, Bytes: doneBytes}
		for _, t := range pending {
			if t.rmdir && t.top {
				phase.Items++ // a merged source counts once its directory is removed
			}
		}
		if slices.ContainsFunc(pending, func(t moveTask) bool { return !t.rmdir }) {
			phase.Count = func(ctx context.Context, popts PlanBuildOptions, sink func(PlanItem)) error {
				popts.FlatDestNames, popts.DereferenceSymlinks = r.opts.FlatDestNames, r.opts.DereferenceSymlinks
				return countMoveTasks(ctx, pending, popts, sink)
			}
		}
		r.opts.OnCopyPhase(phase)
	}
	return r.runTasks(ctx, pending, doneFiles, doneBytes)
}

// moveTask is copy-phase work the rename phase set aside: a copy fallback for a source that could
// not be renamed, or (rmdir) the removal of a merged source directory once the work under it is
// done.
type moveTask struct {
	source                int32 // job source index (Options.enterSource)
	src, dst, destination pathloc.Path
	keepBoth              bool // copy under a free sibling name without asking the resolver again
	rmdir                 bool
	top                   bool // rmdir of a top-level source: one moved item once removed
}

// moveRun carries the per-job settings threaded through every per-entry move.
type moveRun struct {
	opts     Options
	throttle ProgressEmitThrottle
	progress ProgressCallback
	resolver ConflictResolver
	diskWait DiskWaitFunc
	// pending collects the rename phase's set-aside work (collect appends, runTasks drains).
	pending *[]moveTask
}

// moveAndCount moves one entry right away (rename phase, then its own copy phase) and returns the
// updated done totals, counting a completed rename as one item and reporting it through progress.
// Flatten and undo use it; ExecuteMove runs the phases across all sources instead.
func (r moveRun) moveAndCount(ctx context.Context, src, dst, destination pathloc.Path, doneFiles int, doneBytes int64) (int, int64, error) {
	var pending []moveTask
	r.pending = &pending
	moved, err := r.collect(ctx, src, dst, destination, true)
	if err != nil {
		return doneFiles, doneBytes, err
	}
	if moved {
		doneFiles++
		if r.progress != nil {
			r.progress(src.String(), dst.String(), doneFiles, doneBytes)
		}
	}
	return r.runTasks(ctx, pending, doneFiles, doneBytes)
}

// collect is the rename phase for one resolved src and dst. A directory onto an existing
// directory is merged child by child (collectMerge); everything else goes through
// renameSourceForMove, and a source whose rename is not possible is set aside as a copy task.
// moved reports a completed rename (or a merged source directory already removed) that counts as
// one done item. top marks a job source (its merge removal counts as a moved item). destination
// is the directory dst was resolved against.
func (r moveRun) collect(ctx context.Context, src, dst, destination pathloc.Path, top bool) (moved bool, err error) {
	if PathsEquivalent(src, dst) {
		return false, nil
	}
	dstExists, merge, err := moveDestState(ctx, src, dst)
	if err != nil {
		return false, fmt.Errorf("rename %q -> %q: %w", src, dst, err)
	}
	if merge {
		return r.collectMerge(ctx, src, dst, top)
	}
	out, err := renameSourceForMove(ctx, src, dst, dstExists, r.resolver)
	if errors.Is(err, ErrPermissionSkipped) {
		r.opts.itemDone(PlanItem{Src: src, Dst: dst}, ItemSkipped, ReasonPermission)
		return false, nil // user skipped this source after a permission error
	}
	if err != nil {
		return false, fmt.Errorf("rename %q -> %q: %w", src, dst, err)
	}
	switch {
	case out.fallbackCopy:
		*r.pending = append(*r.pending, moveTask{source: r.opts.source, src: src, dst: dst, destination: destination, keepBoth: out.keepBoth})
		return false, nil
	case out.sourceDeleted:
		// Nothing was renamed: the kept destination's cache entries stay its own.
		r.opts.itemDone(PlanItem{Src: src, Dst: dst}, ItemSourceDeleted, ReasonSourceDeleted)
		return true, nil // counts as moved
	case out.renamed:
		if !out.final.IsZero() {
			dst = out.final
		}
		r.opts.journalMoved(ctx, src, dst)
		if out.staged != "" {
			if loc, perr := pathloc.Parse(out.staged); perr == nil {
				_ = removePathRecursive(ctx, loc)
			}
		}
		r.opts.itemDone(PlanItem{Src: src, Dst: dst}, ItemDone, "")
		return true, nil
	}
	reason := ReasonExists
	if out.identical {
		reason = ReasonIdentical
	}
	r.opts.itemDone(PlanItem{Src: src, Dst: dst}, ItemSkipped, reason)
	return false, nil
}

// moveDestState stats dst once and reports whether it exists and whether src and dst are both real
// directories (merge). statEntry does not follow symlinks (local Lstat, sftp Lstat), so a symlink on
// either side reports EntrySymlink: a symlink src is never descended into and a symlink dst (even
// to a directory) is not merged into, so those collisions go through the conflict resolver.
func moveDestState(ctx context.Context, src, dst pathloc.Path) (exists, merge bool, err error) {
	de, err := statEntry(ctx, dst)
	if err != nil {
		if isNotExist(err) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("stat destination %q: %w", dst, err)
	}
	if de.Type != fsbackend.EntryDirectory {
		return true, false, nil
	}
	se, err := statEntry(ctx, src)
	if err != nil {
		return true, false, fmt.Errorf("stat source %q: %w", src, err)
	}
	return true, se.Type == fsbackend.EntryDirectory, nil
}

// collectMerge is the rename phase of moving the children of directory src into the existing
// directory dst, recursing via collect so only colliding subtrees are listed. src is removed
// afterwards when it ended up empty, or by a queued rmdir task once the copy tasks set aside under
// it are done; children the user skipped leave it in place without error.
func (r moveRun) collectMerge(ctx context.Context, src, dst pathloc.Path, top bool) (moved bool, err error) {
	be, err := backendFor(src)
	if err != nil {
		return false, err
	}
	children, err := be.List(ctx, src)
	if err != nil {
		return false, fmt.Errorf("list %q: %w", src, err)
	}
	queued := len(*r.pending)
	for _, c := range children {
		if c.Name == "." || c.Name == ".." {
			continue
		}
		if err := ctx.Err(); err != nil {
			return false, err
		}
		csrc, err := src.Join(c.Name)
		if err != nil {
			return false, err
		}
		cdst, err := dst.Join(c.Name)
		if err != nil {
			return false, err
		}
		// dst as "destination" makes a copy fallback target cdst exactly (dst != cdst).
		if _, err := r.collect(ctx, csrc, cdst, dst, false); err != nil {
			return false, err
		}
	}
	if len(*r.pending) > queued {
		*r.pending = append(*r.pending, moveTask{source: r.opts.source, src: src, dst: dst, rmdir: true, top: top})
		return false, nil
	}
	return removeDirIfEmpty(ctx, be, src, "move")
}

// runTasks is the copy phase: it runs the set-aside tasks in order (a merged directory's rmdir
// follows the work under it) and returns the updated done totals.
func (r moveRun) runTasks(ctx context.Context, tasks []moveTask, doneFiles int, doneBytes int64) (int, int64, error) {
	for _, t := range tasks {
		if err := ctx.Err(); err != nil {
			return doneFiles, doneBytes, err
		}
		r.opts.enterSource(t.source)
		if t.rmdir {
			be, err := backendFor(t.src)
			if err != nil {
				return doneFiles, doneBytes, err
			}
			moved, err := removeDirIfEmpty(ctx, be, t.src, "move")
			if err != nil {
				return doneFiles, doneBytes, err
			}
			if moved && t.top {
				doneFiles++
				if r.progress != nil {
					r.progress(t.src.String(), t.dst.String(), doneFiles, doneBytes)
				}
			}
			continue
		}
		f, b, err := r.copyTask(ctx, t, doneFiles, doneBytes)
		doneFiles += f
		doneBytes += b
		if err != nil {
			return doneFiles, doneBytes, err
		}
	}
	return doneFiles, doneBytes, nil
}

// copyTask copies one set-aside source and removes it; baseFiles and baseBytes offset its progress.
func (r moveRun) copyTask(ctx context.Context, t moveTask, baseFiles int, baseBytes int64) (int, int64, error) {
	orig := r.progress
	fb := r
	fb.opts.OnJournal = nil // the move is journaled once below, not per copied file
	if t.keepBoth {
		fb.resolver = func(context.Context, string, string, FileConflictFacts) (ConflictResolution, error) {
			return ConflictResolution{Action: ActionRename}, nil
		}
	}
	fb.progress = func(s, d string, f int, b int64) {
		if orig != nil {
			orig(s, d, baseFiles+f, baseBytes+b)
		}
	}
	f, b, err := fb.moveCopyFallback(ctx, t.src, t.dst, t.destination)
	if err == nil && !t.keepBoth { // keep-both fallbacks pick their own free name: not journaled
		r.opts.journalMoved(ctx, t.src, t.dst)
	}
	return f, b, err
}

// countMoveTasks walks the sources of the copy tasks the way moveCopyFallback plans them, feeding
// sink each item: the copy phase's totals. opts carries the walk's pacing, item-error handling and
// the job's naming options, which fallbackPlanTarget adjusts per task.
func countMoveTasks(ctx context.Context, tasks []moveTask, opts PlanBuildOptions, sink func(PlanItem)) error {
	feed := func(it PlanItem) error {
		sink(it)
		return nil
	}
	for _, t := range tasks {
		if t.rmdir {
			continue
		}
		planDest, planOpts := fallbackPlanTarget(t.dst, t.destination, opts.FlatDestNames, opts.DereferenceSymlinks)
		walkOpts := opts
		walkOpts.FlatDestNames = planOpts.FlatDestNames
		if err := buildPlan(ctx, []pathloc.Path{t.src}, planDest, true, walkOpts, feed); err != nil {
			return err
		}
	}
	return nil
}

// fallbackPlanTarget is where a copy fallback plans src: dst is the already-resolved destination
// path of src. Unless it is destination itself (a rename-to-new-name move), the plan targets
// dst's parent with flat names so the nested name resolved by ExecuteMove is reproduced exactly.
func fallbackPlanTarget(dst, destination pathloc.Path, flatDestNames, dereference bool) (pathloc.Path, PlanBuildOptions) {
	planOpts := PlanBuildOptions{FlatDestNames: flatDestNames, DereferenceSymlinks: dereference}
	if dst.Equal(destination) {
		return destination, planOpts
	}
	planOpts.FlatDestNames = true
	return dst.Parent(), planOpts
}

// moveCopyFallback copies one source whose rename was not possible to dst (see
// fallbackPlanTarget), then removes it. The plan covers only src.
func (r moveRun) moveCopyFallback(ctx context.Context, src, dst, destination pathloc.Path) (int, int64, error) {
	planDest, planOpts := fallbackPlanTarget(dst, destination, r.opts.FlatDestNames, r.opts.DereferenceSymlinks)
	sources := []pathloc.Path{src}
	plan, _, _, tb, err := BuildCopyPlanWithTotalsCtx(ctx, sources, planDest, planOpts)
	if err != nil {
		return 0, 0, fmt.Errorf("move copy phase plan: %w", err)
	}
	for k := range plan { // planned as a lone source: re-tag with the job source being moved
		plan[k].Source = r.opts.source
	}
	if err := EnsureDiskSpace(r.diskWait, planDest, tb, pathloc.Path{}); err != nil {
		return 0, 0, err
	}
	doneFiles, doneBytes, transferred, err := executeCopyWithPlan(ctx, plan, sources, planDest, r.opts, r.throttle, r.progress, r.resolver, r.diskWait)
	if err != nil {
		return doneFiles, doneBytes, fmt.Errorf("move copy phase: %w", err)
	}
	return finishMoveCopyPhase(ctx, sources, transferred, doneFiles, doneBytes, r.opts.OnRemoveSources)
}

// finishMoveCopyPhase removes transferred sources and any now-empty source directory roots
// after a move's copy-fallback phase has copied a source to its destination. onRemove, when
// non-nil, is called with true before the removal begins and with false once it ends.
func finishMoveCopyPhase(ctx context.Context, sources []pathloc.Path, transferred []pathloc.Path, doneFiles int, doneBytes int64, onRemove func(removing bool)) (int, int64, error) {
	if onRemove != nil {
		onRemove(true)
		defer onRemove(false)
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
