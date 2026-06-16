//go:build linux

package localfs

import (
	"os"

	"golang.org/x/sys/unix"
)

func preallocateDestination(dstFile *os.File, size int64) error {
	if size <= 0 {
		return nil
	}
	fd := int(dstFile.Fd())
	if err := unix.Fallocate(fd, 0, 0, size); err == nil {
		return nil
	}
	return dstFile.Truncate(size)
}
