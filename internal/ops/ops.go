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
	"github.com/paranoidi/paras-commander/internal/localfs"
)

// ProgressEmitThrottle limits how often transfer progress callbacks fire during file copies.
type ProgressEmitThrottle struct {
	MinBytes    int64
	MinInterval time.Duration
}

func effectiveProgressThrottle(t ProgressEmitThrottle) ProgressEmitThrottle {
	if t.MinBytes <= 0 {
		t.MinBytes = int64(config.DefaultProgressEmitMinBytes)
	}
	if t.MinInterval <= 0 {
		t.MinInterval = time.Duration(config.DefaultProgressEmitMinIntervalMS) * time.Millisecond
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

// PathsEquivalent reports whether two filesystem paths refer to the same location.
func PathsEquivalent(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return filepath.Clean(aa) == filepath.Clean(bb)
}

// ResolvedSameAsSource reports whether transferring src into destDir would place
// the item at the same path as src (copy/move onto itself).
func ResolvedSameAsSource(src, destDir string) bool {
	return PathsEquivalent(src, ResolveDestination(src, destDir))
}

// ResolveDestination resolves the full destination path for a source path
// given a user-supplied destination (which may be a directory or a file path).
func ResolveDestination(src, dest string) string {
	destInfo, err := os.Stat(dest)
	if err == nil && destInfo.IsDir() {
		return filepath.Join(dest, filepath.Base(src))
	}
	return dest
}

// RenameFastPath attempts an os.Rename and returns true on success.
// If the rename fails due to a cross-device link, it returns false and nil error
// so the caller can fall back to copy+delete.
// Other errors are returned as-is.
func RenameFastPath(src, dest string) (ok bool, err error) {
	err = os.Rename(src, dest)
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
	Src       string
	Dst       string
	IsDir     bool
	IsSymlink bool
	FileSize  int64
}

// BuildPlan walks sources and creates a flat list of files to copy/move.
func BuildPlan(sources []string, destination string, followDirChildren bool) ([]PlanItem, error) {
	var items []PlanItem
	for _, src := range sources {
		srcInfo, err := os.Lstat(src)
		if err != nil {
			return nil, fmt.Errorf("stat %q: %w", src, err)
		}
		if srcInfo.IsDir() {
			if !followDirChildren {
				continue
			}
			dst := ResolveDestination(src, destination)
			// Walk directory children.
			err := localfs.WalkDirRecursive(src, func(path string, info os.FileInfo) error {
				rel, err := filepath.Rel(src, path)
				if err != nil {
					return fmt.Errorf("compute relative path for %q: %w", path, err)
				}
				childDst := filepath.Join(dst, rel)
				if info.IsDir() {
					items = append(items, PlanItem{
						Src:       path,
						Dst:       childDst,
						IsDir:     true,
						IsSymlink: localfs.IsSymlink(info),
					})
				} else if localfs.IsSymlink(info) {
					items = append(items, PlanItem{
						Src:       path,
						Dst:       childDst,
						IsDir:     false,
						IsSymlink: true,
						FileSize:  0,
					})
				} else if info.Mode().IsRegular() {
					items = append(items, PlanItem{
						Src:       path,
						Dst:       childDst,
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
			dst := ResolveDestination(src, destination)
			switch {
			case localfs.IsSymlink(srcInfo):
				items = append(items, PlanItem{
					Src:       src,
					Dst:       dst,
					IsDir:     false,
					IsSymlink: true,
					FileSize:  0,
				})
			case srcInfo.Mode().IsRegular():
				items = append(items, PlanItem{
					Src:       src,
					Dst:       dst,
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
