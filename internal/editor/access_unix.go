//go:build unix

package editor

import (
	"errors"

	"golang.org/x/sys/unix"
)

// needsWriteElevation reports whether access(2) says path is denied to the current user
// (EACCES/EPERM). EROFS and other errors never elevate: root cannot write there either.
func needsWriteElevation(path string) bool {
	err := unix.Access(path, unix.W_OK)
	return errors.Is(err, unix.EACCES) || errors.Is(err, unix.EPERM)
}
