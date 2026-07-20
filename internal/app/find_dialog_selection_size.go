package app

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// reconcileFindDialogSelectionSizeScans enqueues disk-usage walks for marked directories
// in the find dialog that are missing cache entries.
func (a *App) reconcileFindDialogSelectionSizeScans() {
	if a.diskUsage == nil || a.model.ViewMode != ui.ViewBrowser {
		return
	}
	st := &a.model.FindDialog
	if !st.Open || len(st.MarkedPaths) == 0 {
		a.findDialogSelectionScanFP = ""
		a.findDialogSelectionScanGen = 0
		return
	}
	gen := st.MarkedSelGen()
	if a.findDialogSelectionScanGen == gen {
		return
	}
	a.findDialogSelectionScanGen = gen

	p := a.panelByID(st.PanelID)
	if p.Path.IsRemote() {
		a.findDialogSelectionScanFP = ""
		return
	}
	need := directoriesNeedingScanFromPathIsDir(
		st.PrunedMarkedRoots(),
		st.PathIsDir,
		st.ListingDevice,
		st.ListingDeviceValid,
		a.diskUsage,
		a.config.DiskUsage.DescendIntoMountPoints,
		a.diskUsageIgnore,
	)
	fp := strings.Join(need, "\n")
	if fp == "" {
		a.findDialogSelectionScanFP = ""
		return
	}
	if fp == a.findDialogSelectionScanFP {
		return
	}
	a.findDialogSelectionScanFP = fp
	a.diskUsage.StartScanFromListing(
		need,
		a.diskUsageIgnore,
		st.PanelID,
		diskusage.ListingVolumeGate{
			Enabled: !a.config.DiskUsage.DescendIntoMountPoints && st.ListingDeviceValid,
			RefDev:  st.ListingDevice,
			Valid:   st.ListingDeviceValid,
		},
	)
}
