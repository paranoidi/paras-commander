package app

import (
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/sched"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// reconcileSelectionSizeScans enqueues disk-usage walks for selected directories missing cache
// entries. Which directories still need scanning is computed on a debounced background goroutine
// (armSelectionSizeScanDebounce) since diskusage.DirectoriesNeedingScan stats every selected
// directory not yet cached — for a multi-thousand-directory selection (e.g. select-all) that is
// too much synchronous work for the main goroutine.
func (a *App) reconcileSelectionSizeScans(panelID int) {
	if a.disk.engine == nil || a.model.ViewMode != ui.ViewBrowser {
		return
	}
	p := a.panelByID(panelID)
	if p.SelectedPathCount() == 0 {
		a.selectionSizeScanFP[panelID] = ""
		a.selectionSizeScanGen[panelID] = 0
		a.selectionSizeScanPath[panelID] = ""
		a.selectionSizeScanDebounce[panelID].Invalidate()
		return
	}
	if p.Path.IsRemote() {
		a.selectionSizeScanFP[panelID] = ""
		a.selectionSizeScanGen[panelID] = 0
		a.selectionSizeScanPath[panelID] = ""
		a.selectionSizeScanDebounce[panelID].Invalidate()
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
		a.selectionSizeScanDebounce[panelID].Invalidate()
		return
	}

	// PrunedSelectionRoots returns a live cache slice that later selection edits mutate in
	// place, so it must be copied before crossing the goroutine boundary. EntriesByPath already
	// allocates a fresh map.
	roots := append([]string(nil), p.PrunedSelectionRoots()...)
	byPath := p.EntriesByPath()
	listingDev := p.ListingDevice
	listingDevValid := p.ListingDeviceValid
	screen := a.screen

	a.armSelectionSizeScanDebounce(&a.selectionSizeScanDebounce[panelID], func() []string {
		return diskusage.DirectoriesNeedingScan(
			roots, byPath, listingDev, listingDevValid,
			a.disk.engine, a.config.DiskUsage.DescendIntoMountPoints, a.disk.ignore,
		)
	}, func(need []string) {
		_ = screen.PostEvent(tcell.NewEventInterrupt(selectionScanNeedPayload{PanelID: panelID, Need: need, Gen: gen}))
	})
}

// applySelectionScanNeed applies the result of a background reconcileSelectionSizeScans pass:
// starts the actual (already-async) disk-usage walk for whatever still needs scanning, unless
// the selection has changed again since the scan was computed.
func (a *App) applySelectionScanNeed(d selectionScanNeedPayload) {
	if a.disk.engine == nil {
		return
	}
	p := a.panelByID(d.PanelID)
	if p.SelectionDerivedGen() != d.Gen {
		return
	}
	fp := strings.Join(d.Need, "\n")
	if fp == "" {
		a.selectionSizeScanFP[d.PanelID] = ""
		return
	}
	if fp == a.selectionSizeScanFP[d.PanelID] {
		return
	}
	a.selectionSizeScanFP[d.PanelID] = fp
	a.disk.engine.StartScanFromListing(
		d.Need,
		a.disk.ignore,
		d.PanelID,
		listingVolumeGateForScan(p, a.config.DiskUsage.DescendIntoMountPoints),
	)
}

// armSelectionSizeScanDebounce runs compute (the stat-syscall-heavy "what needs scanning" check)
// on a debounced background goroutine after config.UI.SelectionSizeScanDebounceMS, then calls
// post with its result on that same goroutine. Shared by the panel and find-dialog selection-size
// reconcilers; callers must pass a compute closure over already-snapshotted, goroutine-safe
// inputs (no live UI state), since it runs off the main goroutine.
func (a *App) armSelectionSizeScanDebounce(debounce *sched.Debouncer, compute func() []string, post func(need []string)) {
	delay := time.Duration(a.config.UI.SelectionSizeScanDebounceMS) * time.Millisecond
	debounce.Arm(delay, func() {
		post(compute())
	})
}

func findDialogPathIsDir(pathMeta func(string) (isDir bool, size int64, ok bool)) func(string) bool {
	return func(path string) bool {
		if pathMeta == nil {
			return false
		}
		isDir, _, ok := pathMeta(path)
		return ok && isDir
	}
}

func directoriesNeedingScanFromIsDir(
	pruned []string,
	isDir func(string) bool,
	listingDev uint64,
	listingDevValid bool,
	du interface {
		ByteSize(absPath string) (int64, bool)
		DiskScanExcluded(absPath string, descendIntoMountPoints bool, listingDev uint64, listingDevValid bool, goduIgnore func(string) bool) bool
		MarkExcluded(absPath string)
	},
	descendIntoMountPoints bool,
	goduIgnore func(string) bool,
) []string {
	if du == nil || len(pruned) == 0 || isDir == nil {
		return nil
	}
	var need []string
	for _, path := range pruned {
		if !isDir(path) {
			continue
		}
		if _, ok := du.ByteSize(path); ok {
			continue
		}
		if du.DiskScanExcluded(path, descendIntoMountPoints, listingDev, listingDevValid, goduIgnore) {
			du.MarkExcluded(path)
			continue
		}
		need = append(need, path)
	}
	return need
}
