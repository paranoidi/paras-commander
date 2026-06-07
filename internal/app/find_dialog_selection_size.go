package app

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// reconcileFindDialogSelectionSizeScans enqueues disk-usage walks for marked directories
// in the find dialog that are missing cache entries.
func (a *App) reconcileFindDialogSelectionSizeScans() {
	if !dialog.FindDialogSelectionSizeEnabled {
		a.findDialogSelectionScanFP = ""
		return
	}
	if a.diskUsage == nil || a.model.ViewMode != ui.ViewBrowser {
		return
	}
	st := &a.model.FindDialog
	if !st.Open || len(st.MarkedPaths) == 0 {
		a.findDialogSelectionScanFP = ""
		return
	}
	p := a.panelByID(st.PanelID)
	if p.Path.IsRemote() {
		a.findDialogSelectionScanFP = ""
		return
	}
	paths := make([]string, 0, len(st.MarkedPaths))
	for path, on := range st.MarkedPaths {
		if on {
			paths = append(paths, path)
		}
	}
	need := directoriesNeedingScan(
		panel.PruneNestedPaths(paths),
		nil,
		st.ListingDevice,
		st.ListingDeviceValid,
		a.diskUsage,
		a.config.DiskUsageDescendIntoMountPoints,
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
			Enabled: !a.config.DiskUsageDescendIntoMountPoints && st.ListingDeviceValid,
			RefDev:  st.ListingDevice,
			Valid:   st.ListingDeviceValid,
		},
	)
}
