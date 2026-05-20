// Package ops implements filesystem copy and move operations as used by
// the background job worker. Operations are independent of terminal/UI packages.
package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	isDir, err := destinationIsDir(ctx, dest)
	if err != nil {
		return dest, err
	}
	if isDir {
		child, err := dest.Join(src.Base())
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

	if err := localfs.CopyFile(context.Background(), src, dest, bufSize, opts.PreservePermissions, opts.PreserveTimestamps, false, opts.SyncAfterEachFile, opts.CowFileCloning, nil); err != nil {
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
	Src       pathloc.Path
	Dst       pathloc.Path
	IsDir     bool
	IsSymlink bool
	FileSize  int64
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
	for _, srcLoc := range sources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		dstLoc, err := ResolveDestinationCtx(ctx, srcLoc, destination)
		if err != nil {
			return nil, err
		}
		if srcLoc.IsRemote() || destination.IsRemote() || !useLocalFastPath(srcLoc, dstLoc) {
			srcEnt, err := statEntry(ctx, srcLoc)
			if err != nil {
				return nil, fmt.Errorf("stat %q: %w", srcLoc, err)
			}
			if srcEnt.Type == fsbackend.EntryDirectory {
				if !followDirChildren {
					continue
				}
				if err := walkBackendTree(ctx, srcLoc, dstLoc, &items, afterVisit); err != nil {
					return nil, err
				}
				continue
			}
			if err := afterVisit(srcLoc.String()); err != nil {
				return nil, err
			}
			item, err := planItemFromEntry(srcLoc, dstLoc, srcEnt)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
			continue
		}
		src, err := srcLoc.FilePath()
		if err != nil {
			return nil, err
		}
		srcInfo, err := os.Lstat(src)
		if err != nil {
			return nil, fmt.Errorf("stat %q: %w", src, err)
		}
		if srcInfo.IsDir() {
			if !followDirChildren {
				continue
			}
			dst, err := dstLoc.FilePath()
			if err != nil {
				return nil, err
			}
			err = localfs.WalkDirRecursive(src, func(path string, info os.FileInfo) error {
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
					items = append(items, PlanItem{
						Src:       srcItem,
						Dst:       dstItem,
						IsDir:     true,
						IsSymlink: localfs.IsSymlink(info),
					})
				} else if localfs.IsSymlink(info) {
					items = append(items, PlanItem{
						Src:       srcItem,
						Dst:       dstItem,
						IsDir:     false,
						IsSymlink: true,
						FileSize:  0,
					})
				} else if info.Mode().IsRegular() {
					items = append(items, PlanItem{
						Src:       srcItem,
						Dst:       dstItem,
						IsDir:     false,
						IsSymlink: false,
						FileSize:  localfs.GetFileSize(info),
					})
				} else {
					return fmt.Errorf("unsupported file type for %q (mode %v)", path, info.Mode())
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			if err := afterVisit(src); err != nil {
				return nil, err
			}
			switch {
			case localfs.IsSymlink(srcInfo):
				items = append(items, PlanItem{
					Src:       srcLoc,
					Dst:       dstLoc,
					IsDir:     false,
					IsSymlink: true,
					FileSize:  0,
				})
			case srcInfo.Mode().IsRegular():
				items = append(items, PlanItem{
					Src:       srcLoc,
					Dst:       dstLoc,
					IsDir:     false,
					IsSymlink: false,
					FileSize:  localfs.GetFileSize(srcInfo),
				})
			default:
				return nil, fmt.Errorf("unsupported file type for %q (mode %v)", src, srcInfo.Mode())
			}
		}
	}
	return items, nil
}
