//go:build unix

package localfs

import (
	"errors"
	"io/fs"

	"golang.org/x/sys/unix"
)

// entryAccess reports whether the current process can access path (execute for directories,
// read for files) via a real access(2) check — this honors POSIX ACLs (e.g. auto-mounted
// per-user directories under /media are root-owned 0750 with an ACL entry granting the
// logged-in user rx; a manual permission-bit comparison would miss that). access(2) follows
// symlinks, so a link whose target does not resolve is reported as brokenLink, not denied.
func entryAccess(path string, info fs.FileInfo) (denied, brokenLink bool) {
	mode := uint32(unix.R_OK)
	if info.IsDir() {
		mode = uint32(unix.X_OK)
	}
	err := unix.Access(path, mode)
	if err == nil {
		return false, false
	}
	if info.Mode()&fs.ModeSymlink != 0 &&
		(errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENOTDIR) || errors.Is(err, unix.ELOOP)) {
		return false, true
	}
	return true, false
}
