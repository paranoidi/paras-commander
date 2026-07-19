package panellist

import (
	"github.com/paranoidi/paras-commander/internal/localfs"
)

// EntryOnOtherMount reports whether a directory entry sits on another mount than the panel cwd.
// It compares the device ID captured at listing time (localfs.Entry.Dev) against the listing's
// device — no syscalls: this runs per directory row on every panel paint, and a stat here on a
// copy-saturated NAS mount blocked the UI thread for seconds.
func EntryOnOtherMount(entry localfs.Entry, descendIntoMountPoints bool, listingDev uint64, listingDevValid bool) bool {
	if entry.Type != localfs.EntryDirectory {
		return false
	}
	if descendIntoMountPoints || !listingDevValid || !entry.DevValid {
		return false
	}
	return entry.Dev != listingDev
}
