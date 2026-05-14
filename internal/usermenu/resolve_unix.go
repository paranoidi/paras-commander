//go:build !windows

package usermenu

import (
	"os"

	"golang.org/x/sys/unix"
)

func fileOwnerTrusted(st os.FileInfo) bool {
	sys, ok := st.Sys().(*unix.Stat_t)
	if !ok {
		return true
	}
	uid := uint32(os.Geteuid())
	return sys.Uid == 0 || sys.Uid == uid
}
