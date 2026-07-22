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
	TreeExpanded           bool // row is expanded in tree-mode listing
	TreeLoading            bool // row is a tree-mode directory with an in-flight async child fetch
}

// ResolveFolderIconKind returns the folder icon kind for a directory entry, or false for non-directories.
// Priority: excluded → scanning (disk-pending or tree-loading) → open-in-other-panel →
// tree-expanded → mount → default.
func ResolveFolderIconKind(entry localfs.Entry, ctx FolderIconContext) (theme.FolderIconKind, bool) {
	if entry.Type != localfs.EntryDirectory {
		return 0, false
	}
	if ctx.DiskExcluded && ctx.DiskUsageChrome {
		return theme.FolderIconExcluded, true
	}
	if ctx.DiskPending || ctx.TreeLoading {
		return theme.FolderIconScanning, true
	}
	if EntryOpenInOtherPanel(entry, ctx.OtherPanelPath) {
		return theme.FolderIconOpen, true
	}
	if ctx.TreeExpanded {
		return theme.FolderIconTreeExpanded, true
	}
	if EntryOnOtherMount(entry, ctx.DescendIntoMountPoints, ctx.ListingDev, ctx.ListingDevValid) {
		return theme.FolderIconMount, true
	}
	return theme.FolderIconDefault, true
}
