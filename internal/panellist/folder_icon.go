package panellist

import (
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// FolderIconContext carries panel state needed to resolve directory icon-strip kind.
type FolderIconContext struct {
	OtherPanelPath         string
	DescendIntoMountPoints bool
	ListingDev             uint64
	ListingDevValid        bool
	DiskPending            bool
	DiskExcluded           bool
	DiskUsageChrome        bool // metering active for this panel
}

// ResolveFolderIconKind returns the folder icon kind for a directory entry, or false for non-directories.
// Priority: excluded → scanning → open → mount → default.
func ResolveFolderIconKind(entry localfs.Entry, ctx FolderIconContext) (theme.FolderIconKind, bool) {
	if entry.Type != localfs.EntryDirectory {
		return 0, false
	}
	if ctx.DiskExcluded && ctx.DiskUsageChrome {
		return theme.FolderIconExcluded, true
	}
	if ctx.DiskPending {
		return theme.FolderIconScanning, true
	}
	if EntryOpenInOtherPanel(entry, ctx.OtherPanelPath) {
		return theme.FolderIconOpen, true
	}
	if EntryOnOtherMount(entry, ctx.DescendIntoMountPoints, ctx.ListingDev, ctx.ListingDevValid) {
		return theme.FolderIconMount, true
	}
	return theme.FolderIconDefault, true
}
