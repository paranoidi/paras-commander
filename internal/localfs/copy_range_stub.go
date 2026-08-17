//go:build !linux

package localfs

import (
	"context"
	"os"
)

func tryKernelFileRangeCopy(ctx context.Context, srcFile, dstFile *os.File, size, chunkBytes int64, onWritten func(int64)) (ok bool, err error) {
	_ = ctx
	_ = srcFile
	_ = dstFile
	_ = size
	_ = chunkBytes
	_ = onWritten
	return false, nil
}
