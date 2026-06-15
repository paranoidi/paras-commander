package panellist

import (
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

// EntryOnOtherMount reports whether a directory entry sits on another mount than the panel cwd.
func EntryOnOtherMount(entry localfs.Entry, descendIntoMountPoints bool, listingDev uint64, listingDevValid bool) bool {
	if entry.Type != localfs.EntryDirectory {
		return false
	}
	return diskusage.OnOtherMount(entry.Path, descendIntoMountPoints, listingDev, listingDevValid)
}
