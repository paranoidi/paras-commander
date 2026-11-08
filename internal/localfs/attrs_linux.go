//go:build linux

package localfs

import (
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func statChangeTime(st *syscall.Stat_t) time.Time { return time.Unix(st.Ctim.Unix()) }

// entryBirthTime returns path's birth time, zero when the filesystem does not record one.
func entryBirthTime(path string) time.Time {
	var st unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, unix.STATX_BTIME, &st); err != nil {
		return time.Time{}
	}
	if st.Mask&unix.STATX_BTIME == 0 {
		return time.Time{}
	}
	return time.Unix(st.Btime.Sec, int64(st.Btime.Nsec))
}
