package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// reconcileFindDialogSelectionSizeScans enqueues disk-usage walks for marked directories
// in the find dialog that are missing cache entries. As with reconcileSelectionSizeScans, the
// stat-syscall-heavy "what needs scanning" check runs on a debounced background goroutine
// rather than inline on the main goroutine.
func (a *App) reconcileFindDialogSelectionSizeScans() {
	if a.disk.engine == nil || a.model.ViewMode != ui.ViewBrowser {
		return
	}
	st := &a.model.FindDialog
	if !st.Open || len(st.MarkedPaths) == 0 {
		a.findDialogSelectionScanFP = ""
		a.findDialogSelectionScanGen = 0
		a.findDialogSelectionScanDebounce.Invalidate()
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
		a.findDialogSelectionScanDebounce.Invalidate()
		return
	}

	// PrunedMarkedRoots may return a cache slice reused across rebuilds; copy it before
	// crossing the goroutine boundary. isDir is resolved into a plain snapshot map here (cheap
	// index lookups, no I/O) rather than handing the background goroutine a live callback into
	// find-dialog state, which mutates concurrently while indexing is still running.
	roots := append([]string(nil), st.PrunedMarkedRoots()...)
	isDirFn := findDialogPathIsDir(st.PathMeta)
	pathIsDir := make(map[string]bool, len(roots))
	for _, root := range roots {
		pathIsDir[root] = isDirFn(root)
	}
	listingDev := st.ListingDevice
	listingDevValid := st.ListingDeviceValid
	screen := a.screen

	a.armSelectionSizeScanDebounce(&a.findDialogSelectionScanDebounce, func() []string {
		return directoriesNeedingScanFromIsDir(
			roots, func(path string) bool { return pathIsDir[path] },
			listingDev, listingDevValid,
			a.disk.engine, a.config.DiskUsage.DescendIntoMountPoints, a.disk.ignore,
		)
	}, func(need []string) {
		_ = screen.PostEvent(tcell.NewEventInterrupt(findDialogSelectionScanNeedPayload{Need: need, Gen: gen}))
	})
}

// applyFindDialogSelectionScanNeed applies the result of a background
// reconcileFindDialogSelectionSizeScans pass, unless the marked selection has changed again
// since the scan was computed.
func (a *App) applyFindDialogSelectionScanNeed(d findDialogSelectionScanNeedPayload) {
	if a.disk.engine == nil {
		return
	}
	st := &a.model.FindDialog
	if !st.Open || st.MarkedSelGen() != d.Gen {
		return
	}
	fp := strings.Join(d.Need, "\n")
	if fp == "" {
		a.findDialogSelectionScanFP = ""
		return
	}
	if fp == a.findDialogSelectionScanFP {
		return
	}
	a.findDialogSelectionScanFP = fp
	a.disk.engine.StartScanFromListing(
		d.Need,
		a.disk.ignore,
		st.PanelID,
		diskusage.ListingVolumeGate{
			Enabled: !a.config.DiskUsage.DescendIntoMountPoints && st.ListingDeviceValid,
			RefDev:  st.ListingDevice,
			Valid:   st.ListingDeviceValid,
		},
	)
}
