//go:build linux

package localfs

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestIoctlCloneShouldFallback(t *testing.T) {
	for _, errno := range []unix.Errno{unix.EOPNOTSUPP, unix.ENOSYS, unix.ENOTTY, unix.EXDEV, unix.EINVAL, unix.EPERM} {
		if !ioctlCloneShouldFallback(errno) {
			t.Errorf("%v: want fallback", errno)
		}
	}
	if ioctlCloneShouldFallback(unix.ENOSPC) {
		t.Error("ENOSPC: want hard error")
	}
	if ioctlCloneShouldFallback(nil) {
		t.Error("nil: want no fallback")
	}
}
