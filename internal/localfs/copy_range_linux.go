//go:build linux

package localfs

import (
	"context"
	"errors"
	"io"
	"math"
	"os"

	"golang.org/x/sys/unix"
)

func fileRangeShouldFallback(err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, unix.EXDEV),
		errors.Is(err, unix.EINVAL),
		errors.Is(err, unix.EOPNOTSUPP),
		errors.Is(err, unix.ENOSYS),
		errors.Is(err, unix.ENOTTY):
		return true
	default:
		return false
	}
}

func resetCopyFilesForFallback(srcFile, dstFile *os.File) {
	_, _ = srcFile.Seek(0, io.SeekStart)
	_ = dstFile.Truncate(0)
	_, _ = dstFile.Seek(0, io.SeekStart)
}

// tryKernelFileRangeCopy uses copy_file_range(2) between open file descriptors.
// On unsupported errno it returns (false, nil) so the caller can fall back to read/write.
func tryKernelFileRangeCopy(ctx context.Context, srcFile, dstFile *os.File, size int64, onWritten func(int64)) (ok bool, err error) {
	if size == 0 {
		return true, nil
	}
	var copied int64
	for copied < size {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		remain := size - copied
		chunk := remain
		if chunk > math.MaxInt32 {
			chunk = math.MaxInt32
		}
		n, err := unix.CopyFileRange(int(srcFile.Fd()), nil, int(dstFile.Fd()), nil, int(chunk), 0)
		if err != nil {
			if copied > 0 {
				resetCopyFilesForFallback(srcFile, dstFile)
			}
			if fileRangeShouldFallback(err) {
				return false, nil
			}
			return false, err
		}
		if n == 0 {
			if copied > 0 {
				resetCopyFilesForFallback(srcFile, dstFile)
			}
			return false, nil
		}
		copied += int64(n)
		if onWritten != nil {
			onWritten(int64(n))
		}
	}
	return true, nil
}
