//go:build unix

package localfs

import (
	"io/fs"
	"syscall"
	"time"
)

// entryAttrs are the listing facts read off a FileInfo for the file columns.
type entryAttrs struct {
	owner, group          string
	links                 uint64
	changedAt, accessedAt time.Time
}

// entryAttrsFrom reads the file-column facts from a listing FileInfo (no extra stat; name lookups
// go through the id caches, so this belongs on the listing fetch, not at paint time).
func entryAttrsFrom(info fs.FileInfo) entryAttrs {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return entryAttrs{}
	}
	return entryAttrs{
		owner:      ownerName(st.Uid),
		group:      groupName(st.Gid),
		links:      uint64(st.Nlink),
		changedAt:  statChangeTime(st),
		accessedAt: statAccessTime(st),
	}
}
