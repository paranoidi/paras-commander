//go:build !linux

package localfs

import "os"

func preallocateDestination(dstFile *os.File, size int64) error {
	if size <= 0 {
		return nil
	}
	return dstFile.Truncate(size)
}
