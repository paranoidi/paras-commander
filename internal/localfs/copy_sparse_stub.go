//go:build !linux

package localfs

import (
	"context"
	"io"
	"os"
)

func copySparseUserspace(ctx context.Context, srcFile, dstFile *os.File, size int64, buf []byte, onWritten func(int64)) error {
	srcWrapped := io.Reader(srcFile)
	if ctx != nil {
		srcWrapped = &ctxReader{ctx: ctx, r: srcFile}
	}
	dstWrapped := io.Writer(dstFile)
	if onWritten != nil {
		dstWrapped = &countingWriter{w: dstFile, fn: onWritten}
	}
	_, err := io.CopyBuffer(dstWrapped, srcWrapped, buf)
	return err
}
