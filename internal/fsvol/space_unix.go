//go:build unix

package fsvol

import "golang.org/x/sys/unix"

func volumeBytes(path string) (avail uint64, total uint64, ok bool) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, 0, false
	}
	bs := uint64(st.Bsize)
	avail = uint64(st.Bavail) * bs
	total = uint64(st.Blocks) * bs
	return avail, total, total > 0
}
