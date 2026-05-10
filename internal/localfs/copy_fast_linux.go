//go:build linux

package localfs

import (
	"context"
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func ioctlCloneShouldFallback(err error) bool {
	if err == nil {
		return false
	}
	// Try userspace read/write when the kernel or filesystem does not support cloning.
	switch {
	case errors.Is(err, unix.EOPNOTSUPP),
		errors.Is(err, unix.ENOSYS),
		errors.Is(err, unix.ENOTTY),
		errors.Is(err, unix.EXDEV),
		errors.Is(err, unix.EINVAL):
		return true
	default:
		return false
	}
}

// tryKernelReflinkCopy uses ioctl(FICLONE) to share extents (CoW) when supported (e.g. Btrfs, some XFS).
// On failure with a "not supported" errno it returns (false, nil) so the caller can fall back to read/write.
func tryKernelReflinkCopy(ctx context.Context, srcFile, dstFile *os.File, size int64, onWritten func(int64)) (ok bool, err error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	src := int(srcFile.Fd())
	dst := int(dstFile.Fd())
	if err := unix.IoctlFileClone(dst, src); err != nil {
		if ioctlCloneShouldFallback(err) {
			return false, nil
		}
		return false, err
	}
	if onWritten != nil && size > 0 {
		onWritten(size)
	}
	return true, nil
}
