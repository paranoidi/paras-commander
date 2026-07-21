//go:build unix

package localfs

import (
	"io/fs"

	"golang.org/x/sys/unix"
)

// entryAccessDenied reports whether the current process can access path (execute for
// directories, read for files) via a real access(2) check — this honors POSIX ACLs (e.g.
// auto-mounted per-user directories under /media are root-owned 0750 with an ACL entry
// granting the logged-in user rx; a manual permission-bit comparison would miss that).
func entryAccessDenied(path string, info fs.FileInfo) bool {
	mode := uint32(unix.R_OK)
	if info.IsDir() {
		mode = uint32(unix.X_OK)
	}
	return unix.Access(path, mode) != nil
}
