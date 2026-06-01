package app

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// reconcileSelectionSizeScans enqueues disk-usage walks for selected directories missing cache entries.
func (a *App) reconcileSelectionSizeScans(panelID int) {
	if a.diskUsage == nil || a.model.ViewMode != ui.ViewBrowser {
		return
	}
	p := a.panelByID(panelID)
	if p.SelectedPathCount() == 0 {
		a.selectionSizeScanFP[panelID] = ""
		return
	}
	if p.Path.IsRemote() {
		a.selectionSizeScanFP[panelID] = ""
		return
	}
	need := selectedDirectoriesNeedingScan(
		p,
		a.diskUsage,
		a.config.DiskUsageDescendIntoMountPoints,
		a.diskUsageIgnore,
	)
	fp := strings.Join(need, "\n")
	if fp == "" {
		a.selectionSizeScanFP[panelID] = ""
		return
	}
	if fp == a.selectionSizeScanFP[panelID] {
		return
	}
	a.selectionSizeScanFP[panelID] = fp
	a.diskUsage.StartScanFromListing(
		need,
		a.diskUsageIgnore,
		panelID,
		listingVolumeGateForScan(p, a.config.DiskUsageDescendIntoMountPoints),
	)
}

func selectedDirectoriesNeedingScan(
	p *panel.State,
	du interface {
		ByteSize(absPath string) (int64, bool)
		DiskScanExcluded(absPath string, descendIntoMountPoints bool, listingDev uint64, listingDevValid bool, goduIgnore func(string) bool) bool
	},
	descendIntoMountPoints bool,
	goduIgnore func(string) bool,
) []string {
	if p == nil || du == nil || len(p.SelectedPaths) == 0 {
		return nil
	}
	paths := make([]string, 0, len(p.SelectedPaths))
	for path, on := range p.SelectedPaths {
		if on {
			paths = append(paths, path)
		}
	}
	pruned := panel.PruneNestedPaths(paths)
	byPath := make(map[string]localfs.Entry, len(p.Entries))
	for _, e := range p.Entries {
		byPath[e.Path] = e
	}
	var need []string
	for _, path := range pruned {
		entry, found := byPath[path]
		if !found {
			var err error
			entry, err = localfs.EntryFromPath(path)
			if err != nil || entry.Type != localfs.EntryDirectory {
				continue
			}
		} else if entry.Type != localfs.EntryDirectory {
			continue
		}
		if _, ok := du.ByteSize(path); ok {
			continue
		}
		if du.DiskScanExcluded(path, descendIntoMountPoints, p.ListingDevice, p.ListingDeviceValid, goduIgnore) {
			continue
		}
		need = append(need, path)
	}
	return need
}
