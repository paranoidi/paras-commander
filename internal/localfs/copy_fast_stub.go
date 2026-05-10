//go:build !linux

package localfs

import (
	"context"
	"os"
)

// tryKernelReflinkCopy attempts a kernel-side reflink/clone (Linux FICLONE). Non-Linux builds always return (false, nil).
func tryKernelReflinkCopy(ctx context.Context, srcFile, dstFile *os.File, size int64, onWritten func(int64)) (ok bool, err error) {
	_ = ctx
	_ = srcFile
	_ = dstFile
	_ = size
	_ = onWritten
	return false, nil
}
