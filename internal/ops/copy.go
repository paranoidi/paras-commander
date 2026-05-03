package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// ConflictResolver is called when a destination path already exists.
// It receives source and destination paths plus file metadata and returns true to overwrite,
// false to skip, and an error to abort.
type ConflictResolver func(src, dest string, facts FileConflictFacts) (overwrite bool, err error)

// CopyPlanTotals returns the file count and byte sum for a copy plan after the same
// destination validation as ExecuteCopy. Used to populate job totals before transfer.
func CopyPlanTotals(sources []string, destination string) (totalFiles int, totalBytes int64, err error) {
	if err := prepareCopyDestination(sources, destination); err != nil {
		return 0, 0, err
	}
	plan, err := BuildPlan(sources, destination, true)
	if err != nil {
		return 0, 0, fmt.Errorf("build copy plan: %w", err)
	}
	tf, tb := summarizePlan(plan)
	return tf, tb, nil
}

func prepareCopyDestination(sources []string, destination string) error {
	destInfo, err := os.Stat(destination)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat destination %q: %w", destination, err)
		}
		if len(sources) == 1 {
			srcInfo, statErr := os.Lstat(sources[0])
			if statErr != nil {
				return fmt.Errorf("stat source %q: %w", sources[0], statErr)
			}
			if srcInfo.IsDir() {
				if err := os.MkdirAll(destination, 0o755); err != nil {
					return fmt.Errorf("create destination dir %q: %w", destination, err)
				}
			}
		} else {
			return fmt.Errorf("destination directory %q does not exist", destination)
		}
	} else if !destInfo.IsDir() {
		return fmt.Errorf("destination %q is not a directory", destination)
	}
	return nil
}

func summarizePlan(plan []planItem) (totalFiles int, totalBytes int64) {
	for _, item := range plan {
		totalFiles++
		if !item.isDir && !item.isSymlink {
			totalBytes += item.fileSize
		}
	}
	return totalFiles, totalBytes
}

// ExecuteCopy copies a set of source paths to a destination.
// It handles regular files, directories, and symlinks.
// Returns (doneFiles, doneBytes, error).
// diskWait is optional; when set, regular file copies wait until there is enough free space on the destination volume.
func ExecuteCopy(ctx context.Context, sources []string, destination string, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	if err := prepareCopyDestination(sources, destination); err != nil {
		return 0, 0, err
	}

	plan, err := BuildPlan(sources, destination, true)
	if err != nil {
		return 0, 0, fmt.Errorf("build copy plan: %w", err)
	}

	th := effectiveProgressThrottle(throttle)

	var doneFiles int
	var doneBytes int64
	var bytesSinceEmit int64
	var lastEmit time.Time

	emitProgress := func(srcPath, dstPath string, doneF int, doneB int64, force bool) {
		if progress == nil {
			return
		}
		now := time.Now()
		sinceEmit := time.Duration(0)
		if !lastEmit.IsZero() {
			sinceEmit = now.Sub(lastEmit)
		}
		if force || bytesSinceEmit >= th.MinBytes || (bytesSinceEmit > 0 && !lastEmit.IsZero() && sinceEmit >= th.MinInterval) {
			progress(srcPath, dstPath, doneF, doneB)
			bytesSinceEmit = 0
			lastEmit = now
		}
	}

	for _, item := range plan {
		if err := ctx.Err(); err != nil {
			return doneFiles, doneBytes, err
		}
		if item.isDir && !item.isSymlink {
			if err := os.MkdirAll(item.dst, 0o755); err != nil {
				return doneFiles, doneBytes, fmt.Errorf("create directory %q: %w", item.dst, err)
			}
			doneFiles++
			if progress != nil {
				progress(item.src, item.dst, doneFiles, doneBytes)
			}
			continue
		}

		if item.isSymlink {
			if err := copySymlinkWithConflict(item.src, item.dst, resolver); err != nil {
				return doneFiles, doneBytes, err
			}
			doneFiles++
			if progress != nil {
				progress(item.src, item.dst, doneFiles, doneBytes)
			}
			continue
		}

		srcInfo, err := os.Lstat(item.src)
		if err != nil {
			return doneFiles, doneBytes, fmt.Errorf("stat %q: %w", item.src, err)
		}
		if !srcInfo.Mode().IsRegular() {
			return doneFiles, doneBytes, fmt.Errorf("unsupported file type for %q (mode %v)", item.src, srcInfo.Mode())
		}

		if err := EnsureDiskSpace(diskWait, destination, item.fileSize, item.src); err != nil {
			return doneFiles, doneBytes, err
		}

		copied, err := copyFileWithConflict(ctx, item.src, item.dst, opts, resolver, func(delta int64) {
			doneBytes += delta
			bytesSinceEmit += delta
			emitProgress(item.src, item.dst, doneFiles, doneBytes, false)
		})
		if err != nil {
			return doneFiles, doneBytes, err
		}
		if !copied {
			continue
		}

		doneFiles++
		emitProgress(item.src, item.dst, doneFiles, doneBytes, true)
	}

	return doneFiles, doneBytes, nil
}

func copyFileWithConflict(ctx context.Context, src, dst string, opts Options, resolver ConflictResolver, onWritten func(int64)) (copied bool, err error) {
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

	err = localfs.CopyFile(ctx, src, dst, BufferSize(opts.CopyBufferKiB), opts.PreservePermissions, opts.PreserveTimestamps, false, onWritten)
	if err != nil {
		return false, err
	}
	return true, nil
}

func copySymlinkWithConflict(src, dst string, resolver ConflictResolver) error {
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create parent directory %q: %w", parent, err)
	}

	if _, err := os.Stat(dst); err == nil {
		if resolver == nil {
			return fmt.Errorf("destination %q already exists and no conflict resolver configured", dst)
		}
		facts, err := StatFileConflictFacts(src, dst)
		if err != nil {
			return fmt.Errorf("conflict stat %q %q: %w", src, dst, err)
		}
		overwrite, err := resolver(src, dst, facts)
		if err != nil {
			return err
		}
		if !overwrite {
			return nil
		}
		if err := os.Remove(dst); err != nil {
			return fmt.Errorf("remove existing %q: %w", dst, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat destination %q: %w", dst, err)
	}

	return CopySymlink(src, dst, nil)
}
