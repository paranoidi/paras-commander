package ops

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// lookupMoveBackend resolves the filesystem backend for move dest-stat, rename,
// and rollback. Tests replace this with a blocking fake.
var lookupMoveBackend = backendFor

func moveStat(ctx context.Context, loc pathloc.Path) (fsbackend.Entry, error) {
	be, err := lookupMoveBackend(loc)
	if err != nil {
		return fsbackend.Entry{}, err
	}
	return be.Stat(ctx, loc)
}

func moveDestinationIsDir(ctx context.Context, dest pathloc.Path) (bool, error) {
	ent, err := moveStat(ctx, dest)
	if err != nil {
		if isNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return ent.Type == fsbackend.EntryDirectory, nil
}

// resolveMoveDestination is ResolveDestinationNamed using the job context and
// lookupMoveBackend so a canceled dest Stat unwinds the rename phase.
func resolveMoveDestination(ctx context.Context, dest pathloc.Path, name string) (pathloc.Path, error) {
	if err := ctx.Err(); err != nil {
		return pathloc.Path{}, err
	}
	isDir, err := moveDestinationIsDir(ctx, dest)
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
		be, err := lookupMoveBackend(src)
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
		_, statErr := moveStat(ctx, cand)
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

// renameSourceForMove handles conflict resolution then RenameFastPath for one source.
// Returns renamed when the path was moved, skipped when the user chose not to overwrite,
// fallbackCopy when cross-device (or non-fast) rename requires copy+delete for the batch.
// staged is the parked original destination after an overwrite; empty otherwise. dstExists is the
// caller's stat of dst.
func renameSourceForMove(ctx context.Context, src, dst pathloc.Path, dstExists bool, resolver ConflictResolver) (renamed, skipped, fallbackCopy bool, staged string, err error) {
	if err := ctx.Err(); err != nil {
		return false, false, false, "", err
	}
	if !dstExists {
		renamed, skipped, fallbackCopy, err = renameFastPathOrFallback(ctx, src, dst)
		return renamed, skipped, fallbackCopy, "", err
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
	renamed, skipped, fallbackCopy, err = renameFastPathOrFallback(ctx, src, dst)
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

func renameFastPathOrFallback(ctx context.Context, src, dst pathloc.Path) (renamed, skipped, fallbackCopy bool, err error) {
	ok, err := RenameFastPathCtx(ctx, src, dst)
	if err != nil {
		return false, false, false, err
	}
	if !ok {
		return false, false, true, nil
	}
	return true, false, false, nil
}

// ExecuteMove moves each source to destination, one at a time, in the style of mc: try the O(1)
// rename first and only plan, size and copy+delete a source whose rename cannot be used (cross
// device or cross host). Earlier renames are never rolled back when a later source fails or needs
// the copy fallback. doneFiles counts top-level sources moved by rename plus the items copied for
// fallback sources.
func ExecuteMove(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	var nameRoot pathloc.Path
	if !opts.FlatDestNames {
		nameRoot = TransferNameRoot(sources)
	}
	r := moveRun{opts: opts, throttle: throttle, progress: progress, resolver: resolver, diskWait: diskWait}
	var doneFiles int
	var doneBytes int64
	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			return doneFiles, doneBytes, err
		}
		name := TransferDestName(src, nameRoot)
		dst, err := resolveMoveDestination(ctx, destination, name)
		if err != nil {
			return doneFiles, doneBytes, err
		}
		if strings.ContainsAny(name, `/\`) {
			if err := ensureParentDirs(ctx, dst); err != nil {
				return doneFiles, doneBytes, fmt.Errorf("create parent for %q: %w", dst, err)
			}
		}
		doneFiles, doneBytes, err = r.moveAndCount(ctx, src, dst, destination, doneFiles, doneBytes)
		if err != nil {
			return doneFiles, doneBytes, err
		}
	}
	return doneFiles, doneBytes, nil
}

// moveRun carries the per-job settings threaded through every per-entry move.
type moveRun struct {
	opts     Options
	throttle ProgressEmitThrottle
	progress ProgressCallback
	resolver ConflictResolver
	diskWait DiskWaitFunc
}

// moveAndCount runs moveOne and returns the updated done totals, counting a completed rename as one
// item and reporting it through progress.
func (r moveRun) moveAndCount(ctx context.Context, src, dst, destination pathloc.Path, doneFiles int, doneBytes int64) (int, int64, error) {
	files, bytes, moved, err := r.moveOne(ctx, src, dst, destination, doneFiles, doneBytes)
	doneFiles += files
	doneBytes += bytes
	if err != nil {
		return doneFiles, doneBytes, err
	}
	if moved {
		doneFiles++
		if r.progress != nil {
			r.progress(src.String(), dst.String(), doneFiles, doneBytes)
		}
	}
	return doneFiles, doneBytes, nil
}

// moveOne moves one resolved src to dst. A directory onto an existing directory is merged child by
// child (mergeMoveDir); everything else goes through renameSourceForMove and, when the rename is not
// possible, moveCopyFallback. moved reports a completed rename (or fully merged and removed source
// directory) that counts as one done item; files and bytes are items copied by fallbacks. baseFiles
// and baseBytes offset fallback progress. destination is the directory dst was resolved against.
func (r moveRun) moveOne(ctx context.Context, src, dst, destination pathloc.Path, baseFiles int, baseBytes int64) (files int, bytes int64, moved bool, err error) {
	if PathsEquivalent(src, dst) {
		return 0, 0, false, nil
	}
	dstExists, merge, err := moveDestState(ctx, src, dst)
	if err != nil {
		return 0, 0, false, fmt.Errorf("rename %q -> %q: %w", src, dst, err)
	}
	if merge {
		return r.mergeMoveDir(ctx, src, dst, baseFiles, baseBytes)
	}
	renamed, _, needCopy, staged, err := renameSourceForMove(ctx, src, dst, dstExists, r.resolver)
	if err != nil {
		return 0, 0, false, fmt.Errorf("rename %q -> %q: %w", src, dst, err)
	}
	switch {
	case needCopy:
		orig := r.progress
		fb := r
		fb.progress = func(s, d string, f int, b int64) {
			if orig != nil {
				orig(s, d, baseFiles+f, baseBytes+b)
			}
		}
		f, b, err := fb.moveCopyFallback(ctx, src, dst, destination)
		return f, b, false, err
	case renamed:
		if staged != "" {
			if loc, perr := pathloc.Parse(staged); perr == nil {
				_ = removePathRecursive(ctx, loc)
			}
		}
		return 0, 0, true, nil
	}
	return 0, 0, false, nil
}

// moveDestState stats dst once and reports whether it exists and whether src and dst are both real
// directories (merge). moveStat does not follow symlinks (local Lstat, sftp Lstat), so a symlink on
// either side reports EntrySymlink: a symlink src is never descended into and a symlink dst (even
// to a directory) is not merged into, so those collisions go through the conflict resolver.
func moveDestState(ctx context.Context, src, dst pathloc.Path) (exists, merge bool, err error) {
	de, err := moveStat(ctx, dst)
	if err != nil {
		if isNotExist(err) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("stat destination %q: %w", dst, err)
	}
	if de.Type != fsbackend.EntryDirectory {
		return true, false, nil
	}
	se, err := moveStat(ctx, src)
	if err != nil {
		return true, false, fmt.Errorf("stat source %q: %w", src, err)
	}
	return true, se.Type == fsbackend.EntryDirectory, nil
}

// mergeMoveDir moves the children of directory src into the existing directory dst, recursing via
// moveOne so only colliding subtrees are listed. src is removed afterwards when it ended up empty;
// children the user skipped leave it in place without error.
func (r moveRun) mergeMoveDir(ctx context.Context, src, dst pathloc.Path, baseFiles int, baseBytes int64) (files int, bytes int64, moved bool, err error) {
	be, err := lookupMoveBackend(src)
	if err != nil {
		return 0, 0, false, err
	}
	children, err := be.List(ctx, src)
	if err != nil {
		return 0, 0, false, fmt.Errorf("list %q: %w", src, err)
	}
	for _, c := range children {
		if c.Name == "." || c.Name == ".." {
			continue
		}
		if err := ctx.Err(); err != nil {
			return files, bytes, false, err
		}
		csrc, err := src.Join(c.Name)
		if err != nil {
			return files, bytes, false, err
		}
		cdst, err := dst.Join(c.Name)
		if err != nil {
			return files, bytes, false, err
		}
		// dst as "destination" makes a copy fallback target cdst exactly (dst != cdst).
		f, b, _, err := r.moveOne(ctx, csrc, cdst, dst, baseFiles+files, baseBytes+bytes)
		files += f
		bytes += b
		if err != nil {
			return files, bytes, false, err
		}
	}
	moved, err = removeDirIfEmpty(ctx, be, src, "move")
	return files, bytes, moved, err
}

// moveCopyFallback copies one source whose rename was not possible to dst, then removes it.
// The plan covers only src. dst is the already-resolved destination path of src: unless it is
// destination itself (a rename-to-new-name move), the plan targets dst's parent with flat names
// so the nested name resolved by ExecuteMove is reproduced exactly.
func (r moveRun) moveCopyFallback(ctx context.Context, src, dst, destination pathloc.Path) (int, int64, error) {
	planOpts := PlanBuildOptions{FlatDestNames: r.opts.FlatDestNames, DereferenceSymlinks: r.opts.DereferenceSymlinks}
	planDest := destination
	if !dst.Equal(destination) {
		planDest = dst.Parent()
		planOpts.FlatDestNames = true
	}
	sources := []pathloc.Path{src}
	plan, _, _, tb, err := BuildCopyPlanWithTotalsCtx(ctx, sources, planDest, planOpts)
	if err != nil {
		return 0, 0, fmt.Errorf("move copy phase plan: %w", err)
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
// non-nil, is called before the removal loop begins.
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
