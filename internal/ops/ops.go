// Package ops implements filesystem copy and move operations as used by
// the background job worker. Operations are independent of terminal/UI packages.
package ops

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// ProgressEmitThrottle limits how often transfer progress callbacks fire during file copies.
type ProgressEmitThrottle struct {
	MinBytes    int64
	MinInterval time.Duration
}

func effectiveProgressThrottle(t ProgressEmitThrottle) ProgressEmitThrottle {
	if t.MinBytes <= 0 {
		t.MinBytes = int64(config.DefaultWorkerProgressMinBytes)
	}
	if t.MinInterval <= 0 {
		t.MinInterval = time.Duration(config.DefaultWorkerProgressMinIntervalMS) * time.Millisecond
	}
	return t
}

// Options controls operation behavior.
type Options struct {
	PreservePermissions bool
	PreserveTimestamps  bool
	CopyBufferKiB       int
	// SyncAfterEachFile fsyncs each destination file after copy (durable but slow for many files).
	SyncAfterEachFile bool
	// DiskSpaceCheckMinFileBytes runs per-file EnsureDiskSpace only when the source file is at least this large.
	// Zero means check before every file (legacy behavior).
	DiskSpaceCheckMinFileBytes int64
	// CowFileCloning enables Linux FICLONE (CoW) when supported, like Midnight Commander's "file cloning".
	CowFileCloning bool
	// CopyFileRange tries Linux copy_file_range(2) after FICLONE before userspace read/write.
	CopyFileRange bool
	// SparseFileCopy preserves sparse regions on Linux (SEEK_DATA/SEEK_HOLE).
	SparseFileCopy bool
	// PreallocateDestination reserves destination space before copy when source size is known.
	PreallocateDestination bool
	// PreallocateMinFileBytes applies preallocation only when source size is at least this value (0 = always).
	PreallocateMinFileBytes int64
	// SyncAtJobEnd fsyncs copied local files once after the job when SyncAfterEachFile is false.
	SyncAtJobEnd bool
	// SyncMinFileKiB skips fsync for files smaller than this threshold (0 = no minimum).
	SyncMinFileKiB int
	// FlatDestNames resolves every source to dest/<basename> (flatten jobs) instead of
	// batch-relative names below the sources' common parent (see TransferNameRoot).
	FlatDestNames bool
}

// DefaultOptions returns operation defaults aligned with config.Default().Operations.
func DefaultOptions() Options {
	o := config.Default().Operations
	return Options{
		PreservePermissions:        o.PreservePermissions,
		PreserveTimestamps:         o.PreserveTimestamps,
		CopyBufferKiB:              o.CopyBufferKiB,
		SyncAfterEachFile:          o.SyncAfterEachFile,
		DiskSpaceCheckMinFileBytes: o.DiskSpaceCheckMinFileBytes,
		CowFileCloning:             o.CowFileCloning,
		CopyFileRange:              o.CopyFileRange,
		SparseFileCopy:             o.SparseFileCopy,
		PreallocateDestination:     o.PreallocateDestination,
		PreallocateMinFileBytes:    o.PreallocateMinFileBytes,
		SyncAtJobEnd:               o.SyncAtJobEnd,
		SyncMinFileKiB:             o.SyncMinFileKiB,
	}
}

// SyncFileNow reports whether a copied file should be fsync'd immediately after write.
func (o Options) SyncFileNow(size int64) bool {
	if !o.SyncAfterEachFile {
		return false
	}
	return o.syncFileMeetsMinSize(size)
}

// SyncFileDeferred reports whether a copied local file should be fsync'd at job end.
func (o Options) SyncFileDeferred(size int64) bool {
	if o.SyncAfterEachFile || !o.SyncAtJobEnd {
		return false
	}
	return o.syncFileMeetsMinSize(size)
}

func (o Options) syncFileMeetsMinSize(size int64) bool {
	if o.SyncMinFileKiB <= 0 {
		return true
	}
	return size >= int64(o.SyncMinFileKiB)*1024
}

// LocalCopyFileOpts builds localfs.CopyFileOpts for a single file copy.
func (o Options) LocalCopyFileOpts(buf []byte) localfs.CopyFileOpts {
	return localfs.CopyFileOpts{
		Buf:            buf,
		CopyFileRange:  o.CopyFileRange,
		SparseCopy:     o.SparseFileCopy,
		Preallocate:    o.PreallocateDestination,
		PreallocateMin: o.PreallocateMinFileBytes,
		SyncPerFile:    o.SyncAfterEachFile,
		SyncMinFileKiB: o.SyncMinFileKiB,
	}
}

// PathsEquivalent reports whether two locations refer to the same path.
func PathsEquivalent(a, b pathloc.Path) bool {
	return a.Equal(b)
}

// ResolvedSameAsSource reports whether transferring src into destDir would place
// the item at the same path as src (copy/move onto itself).
func ResolvedSameAsSource(src, destDir pathloc.Path) bool {
	return PathsEquivalent(src, ResolveDestination(src, destDir))
}

// SelfTargetCount reports how many of sources would resolve onto themselves when
// transferred into destDir, using the same batch-relative naming as BuildPlan.
// flatDestNames mirrors the "Flatten into destination" transfer option: when true,
// naming is basename-only (zero root) instead of batch-relative structure.
func SelfTargetCount(sources []pathloc.Path, destDir pathloc.Path, flatDestNames bool) int {
	var root pathloc.Path
	if !flatDestNames {
		root = TransferNameRoot(sources)
	}
	n := 0
	for _, src := range sources {
		if PathsEquivalent(src, ResolveDestinationNamed(destDir, TransferDestName(src, root))) {
			n++
		}
	}
	return n
}

// TransferNameRoot returns the deepest common ancestor of the sources' parent
// directories. Destination names for a batch transfer are taken relative to this
// root so a selection spanning multiple directories keeps its structure under
// the destination. Zero when sources are empty or mix schemes/hosts (callers
// fall back to basename naming).
func TransferNameRoot(sources []pathloc.Path) pathloc.Path {
	parents := make([]pathloc.Path, 0, len(sources))
	for _, src := range sources {
		parents = append(parents, src.Parent())
	}
	root, _, ok := pathloc.CommonParent(parents)
	if !ok {
		return pathloc.Path{}
	}
	return root
}

// TransferDestName returns the destination child name for src in a batch rooted
// at root (from TransferNameRoot): src's path below root, or its basename when
// root is zero or src is not under it. The result may span multiple path
// segments; pathloc.Path.Join accepts it.
func TransferDestName(src, root pathloc.Path) string {
	if !root.IsZero() && !src.Equal(root) && src.HasPrefix(root) {
		rel := strings.TrimPrefix(src.String(), root.String())
		rel = strings.TrimLeft(rel, `/\`)
		if rel != "" {
			return rel
		}
	}
	return src.Base()
}

// DestinationUnderSource reports whether resolvedDest lies inside src (strict descendant).
func DestinationUnderSource(src, resolvedDest pathloc.Path) bool {
	if src.IsZero() || resolvedDest.IsZero() || src.Equal(resolvedDest) {
		return false
	}
	return pathloc.EqualOrUnder(src, resolvedDest)
}

// ResolveDestination resolves the full destination path for a source path
// given a user-supplied destination (which may be a directory or a file path).
func ResolveDestination(src, dest pathloc.Path) pathloc.Path {
	resolved, err := ResolveDestinationCtx(context.Background(), src, dest)
	if err != nil {
		return dest
	}
	return resolved
}

// ResolveDestinationCtx is ResolveDestination with backend Stat for remote paths.
func ResolveDestinationCtx(ctx context.Context, src, dest pathloc.Path) (pathloc.Path, error) {
	return resolveDestinationNamedCtx(ctx, dest, src.Base())
}

// ResolveDestinationNamed is ResolveDestination with an explicit child name
// (see TransferDestName) instead of src's basename.
func ResolveDestinationNamed(dest pathloc.Path, name string) pathloc.Path {
	resolved, err := resolveDestinationNamedCtx(context.Background(), dest, name)
	if err != nil {
		return dest
	}
	return resolved
}

func resolveDestinationNamedCtx(ctx context.Context, dest pathloc.Path, name string) (pathloc.Path, error) {
	isDir, err := destinationIsDir(ctx, dest)
	if err != nil {
		return dest, err
	}
	if isDir {
		child, err := dest.Join(name)
		if err != nil {
			return dest, err
		}
		return child, nil
	}
	return dest, nil
}

// DestinationIsDirAtEnqueue reports whether dest names an existing directory,
// using the same Stat semantics as ResolveDestination. Call once when
// queueing a job so UI listing markers avoid per-row Stat on the destination.
func DestinationIsDirAtEnqueue(dest pathloc.Path) bool {
	isDir, err := destinationIsDir(context.Background(), dest)
	return err == nil && isDir
}

// RenameFastPath attempts a backend rename and returns true on success.
// If the rename fails due to a cross-device link (local), it returns false and nil error
// so the caller can fall back to copy+delete.
// Other errors are returned as-is.
func RenameFastPath(src, dest pathloc.Path) (ok bool, err error) {
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
		if err := be.Rename(context.Background(), src, dest); err == nil {
			return true, nil
		}
		return false, err
	}
	srcHost, err := src.FilePath()
	if err != nil {
		return false, err
	}
	destHost, err := dest.FilePath()
	if err != nil {
		return false, err
	}
	err = os.Rename(srcHost, destHost)
	if err == nil {
		return true, nil
	}
	if localfs.IsCrossDeviceRenameError(err) {
		return false, nil
	}
	return false, err
}

// BufferSize returns the copy buffer size in bytes from the KiB option.
func BufferSize(copyBufferKiB int) int {
	if copyBufferKiB <= 0 {
		return config.DefaultCopyBufferKiB * 1024
	}
	return copyBufferKiB * 1024
}

// ProgressCallback is called during copy/move to report progress.
// destPath is the target path inside the destination tree (used for UI-relative labels).
type ProgressCallback func(sourcePath, destPath string, doneFiles int, doneBytes int64)

// CopyRegular copies a regular file from src to dest.
// dest is the exact target path (not a directory).
func CopyRegular(src, dest string, opts Options, progress ProgressCallback) error {
	bufSize := BufferSize(opts.CopyBufferKiB)
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("stat source %q: %w", src, err)
	}

	if err := localfs.CopyFile(context.Background(), src, dest, bufSize, opts.PreservePermissions, opts.PreserveTimestamps, false, opts.CowFileCloning, opts.LocalCopyFileOpts(nil), nil); err != nil {
		return err
	}

	if progress != nil {
		progress(src, dest, 1, srcInfo.Size())
	}
	return nil
}

// CopySymlink recreates a symlink at dest pointing to the same target as src.
func CopySymlink(src, dest string, progress ProgressCallback) error {
	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf("read symlink %q: %w", src, err)
	}
	if err := os.Symlink(target, dest); err != nil {
		return fmt.Errorf("create symlink %q -> %q: %w", dest, target, err)
	}
	if progress != nil {
		progress(src, dest, 1, 0)
	}
	return nil
}

// PlanItem describes a single file to copy/move.
type PlanItem struct {
	Src        pathloc.Path
	Dst        pathloc.Path
	IsDir      bool
	IsSymlink  bool
	FileSize   int64
	Mode       fs.FileMode // source permission bits when known; zero uses mkdir default
	AccessTime time.Time   // zero when unknown (remote backends may omit atime)
	ModTime    time.Time   // zero when unknown
}

// BuildPlan walks sources and creates a flat list of files to copy/move.
func BuildPlan(sources []pathloc.Path, destination pathloc.Path, followDirChildren bool) ([]PlanItem, error) {
	return BuildPlanCtx(context.Background(), sources, destination, followDirChildren, PlanBuildOptions{})
}

// BuildPlanCtx is BuildPlan with context cancellation and optional walk hooks.
func BuildPlanCtx(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, followDirChildren bool, opts PlanBuildOptions) ([]PlanItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var items []PlanItem
	var visitCount int
	afterVisit := func(path string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if opts.OnPath != nil {
			if err := opts.OnPath(path); err != nil {
				return err
			}
		}
		visitCount++
		if opts.Yield != nil && opts.YieldEveryN > 0 && visitCount%opts.YieldEveryN == 0 {
			opts.Yield()
		}
		return nil
	}
	var nameRoot pathloc.Path
	if !opts.FlatDestNames {
		nameRoot = TransferNameRoot(sources)
	}
	for _, srcLoc := range sources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		dstLoc, err := resolveDestinationNamedCtx(ctx, destination, TransferDestName(srcLoc, nameRoot))
		if err != nil {
			return nil, err
		}
		if DestinationUnderSource(srcLoc, dstLoc) {
			return nil, fmt.Errorf("cannot copy %s into a subdirectory of itself", srcLoc)
		}
		if srcLoc.IsRemote() || destination.IsRemote() || !useLocalFastPath(srcLoc, dstLoc) {
			if err := planRemoteSource(ctx, srcLoc, dstLoc, followDirChildren, &items, afterVisit); err != nil {
				return nil, err
			}
			continue
		}
		if err := planLocalSource(srcLoc, dstLoc, followDirChildren, &items, afterVisit); err != nil {
			return nil, err
		}
	}
	return items, nil
}

// planRemoteSource plans one BuildPlanCtx source through the fsbackend stat/walk path: used
// when either endpoint is remote or the local os fast path isn't available for this src/dst
// pair. Stats srcLoc, then either walks a directory tree via walkBackendTree or appends a
// single planItemFromEntry to *items.
func planRemoteSource(ctx context.Context, srcLoc, dstLoc pathloc.Path, followDirChildren bool, items *[]PlanItem, afterVisit func(string) error) error {
	srcEnt, err := statEntry(ctx, srcLoc)
	if err != nil {
		return fmt.Errorf("stat %q: %w", srcLoc, err)
	}
	if srcEnt.Type == fsbackend.EntryDirectory {
		if !followDirChildren {
			return nil
		}
		return walkBackendTree(ctx, srcLoc, dstLoc, items, afterVisit)
	}
	if err := afterVisit(srcLoc.String()); err != nil {
		return err
	}
	item, err := planItemFromEntry(srcLoc, dstLoc, srcEnt)
	if err != nil {
		return err
	}
	*items = append(*items, item)
	return nil
}

// planLocalSource plans one BuildPlanCtx source through the local os.Lstat/WalkDirRecursive
// fast path (both endpoints local and useLocalFastPath allows it). Appends resulting items
// to *items.
func planLocalSource(srcLoc, dstLoc pathloc.Path, followDirChildren bool, items *[]PlanItem, afterVisit func(string) error) error {
	src, err := srcLoc.FilePath()
	if err != nil {
		return err
	}
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("stat %q: %w", src, err)
	}
	if srcInfo.IsDir() {
		if !followDirChildren {
			return nil
		}
		dst, err := dstLoc.FilePath()
		if err != nil {
			return err
		}
		return localfs.WalkDirRecursive(src, func(path string, info os.FileInfo) error {
			if err := afterVisit(path); err != nil {
				return err
			}
			rel, err := filepath.Rel(src, path)
			if err != nil {
				return fmt.Errorf("compute relative path for %q: %w", path, err)
			}
			childDstHost := filepath.Join(dst, rel)
			srcItem, err := pathloc.File(path)
			if err != nil {
				return err
			}
			dstItem, err := pathloc.File(childDstHost)
			if err != nil {
				return err
			}
			if info.IsDir() {
				*items = append(*items, planItemFromLocalInfo(srcItem, dstItem, info, true, localfs.IsSymlink(info)))
			} else if localfs.IsSymlink(info) {
				*items = append(*items, planItemFromLocalInfo(srcItem, dstItem, info, false, true))
			} else if info.Mode().IsRegular() {
				*items = append(*items, planItemFromLocalInfo(srcItem, dstItem, info, false, false))
			} else {
				return fmt.Errorf("unsupported file type for %q (mode %v)", path, info.Mode())
			}
			return nil
		})
	}
	if err := afterVisit(src); err != nil {
		return err
	}
	switch {
	case localfs.IsSymlink(srcInfo):
		*items = append(*items, planItemFromLocalInfo(srcLoc, dstLoc, srcInfo, false, true))
	case srcInfo.Mode().IsRegular():
		*items = append(*items, planItemFromLocalInfo(srcLoc, dstLoc, srcInfo, false, false))
	default:
		return fmt.Errorf("unsupported file type for %q (mode %v)", src, srcInfo.Mode())
	}
	return nil
}

func planItemFromLocalInfo(src, dst pathloc.Path, info os.FileInfo, isDir, isSymlink bool) PlanItem {
	atime, mtime := localfs.FileTimes(info)
	return PlanItem{
		Src:        src,
		Dst:        dst,
		IsDir:      isDir,
		IsSymlink:  isSymlink,
		FileSize:   localfs.GetFileSize(info),
		Mode:       info.Mode().Perm(),
		AccessTime: atime,
		ModTime:    mtime,
	}
}
