//go:build unix

package localfs

import (
	"io/fs"
	"syscall"
)

// entryDevice extracts st_dev from a listing FileInfo (no extra syscall).
func entryDevice(info fs.FileInfo) (uint64, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(st.Dev), true
}
