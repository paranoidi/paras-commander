//go:build !windows

package filetrust

import (
	"os"
	"os/user"
	"strconv"
	"syscall"
)

func checkInfo(st os.FileInfo) string {
	// os.Lstat fills Sys() with *syscall.Stat_t (not x/sys/unix's type).
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return evaluate(st.Mode(), uint32(os.Geteuid()), 0, os.Geteuid(), func(uint32, uint32) bool { return false })
	}
	return evaluate(st.Mode(), sys.Uid, sys.Gid, os.Geteuid(), isPrivateGroup)
}

// isPrivateGroup reports whether gid names a group called after the user uid. Lookup failures
// count as shared (untrusted).
func isPrivateGroup(uid, gid uint32) bool {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return false
	}
	g, err := user.LookupGroupId(strconv.FormatUint(uint64(gid), 10))
	if err != nil {
		return false
	}
	return g.Name == u.Username
}
