//go:build linux

package localfs

import (
	"io/fs"
	"syscall"
	"time"
)

func fileAccessTime(info fs.FileInfo, fallback time.Time) time.Time {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fallback
	}
	return time.Unix(st.Atim.Sec, st.Atim.Nsec)
}
