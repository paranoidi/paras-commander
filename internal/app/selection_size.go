package app

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// reconcileSelectionSizeScans enqueues disk-usage walks for selected directories missing cache entries.
func (a *App) reconcileSelectionSizeScans(panelID int) {
	if a.disk.engine == nil || a.model.ViewMode != ui.ViewBrowser {
		return
	}
	p := a.panelByID(panelID)
	if p.SelectedPathCount() == 0 {
		a.selectionSizeScanFP[panelID] = ""
		a.selectionSizeScanGen[panelID] = 0
		a.selectionSizeScanPath[panelID] = ""
		return
	}
	if p.Path.IsRemote() {
		a.selectionSizeScanFP[panelID] = ""
		a.selectionSizeScanGen[panelID] = 0
		a.selectionSizeScanPath[panelID] = ""
		return
	}
	path := p.Path.String()
	gen := p.SelectionDerivedGen()
	if a.selectionSizeScanGen[panelID] == gen && a.selectionSizeScanPath[panelID] == path {
		return
	}
	a.selectionSizeScanGen[panelID] = gen
	a.selectionSizeScanPath[panelID] = path
	if !p.SelectionHasDirs() {
		a.selectionSizeScanFP[panelID] = ""
		return
	}

	byPath := p.EntriesByPath()
	need := diskusage.DirectoriesNeedingScan(
		p.PrunedSelectionRoots(),
		byPath,
		p.ListingDevice,
		p.ListingDeviceValid,
		a.disk.engine,
		a.config.DiskUsage.DescendIntoMountPoints,
		a.disk.ignore,
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
	a.disk.engine.StartScanFromListing(
		need,
		a.disk.ignore,
		panelID,
		listingVolumeGateForScan(p, a.config.DiskUsage.DescendIntoMountPoints),
	)
}

func directoriesNeedingScanFromPathIsDir(
	pruned []string,
	pathIsDir map[string]bool,
	listingDev uint64,
	listingDevValid bool,
	du interface {
		ByteSize(absPath string) (int64, bool)
		DiskScanExcluded(absPath string, descendIntoMountPoints bool, listingDev uint64, listingDevValid bool, goduIgnore func(string) bool) bool
	},
	descendIntoMountPoints bool,
	goduIgnore func(string) bool,
) []string {
	if du == nil || len(pruned) == 0 {
		return nil
	}
	var need []string
	for _, path := range pruned {
		if pathIsDir == nil || !pathIsDir[path] {
			continue
		}
		if _, ok := du.ByteSize(path); ok {
			continue
		}
		if du.DiskScanExcluded(path, descendIntoMountPoints, listingDev, listingDevValid, goduIgnore) {
			continue
		}
		need = append(need, path)
	}
	return need
}
