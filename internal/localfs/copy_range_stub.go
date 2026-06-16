//go:build !linux

package localfs

import (
	"context"
	"os"
)

func tryKernelFileRangeCopy(ctx context.Context, srcFile, dstFile *os.File, size int64, onWritten func(int64)) (ok bool, err error) {
	_ = ctx
	_ = srcFile
	_ = dstFile
	_ = size
	_ = onWritten
	return false, nil
}
