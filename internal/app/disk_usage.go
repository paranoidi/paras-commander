package app

import (
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func listingVolumeGateForScan(p *panel.State, descendIntoMountPoints bool) diskusage.ListingVolumeGate {
	return diskusage.ListingVolumeGate{
		Enabled: !descendIntoMountPoints && p.ListingDeviceValid,
		RefDev:  p.ListingDevice,
		Valid:   p.ListingDeviceValid,
	}
}

func (a *App) pollDiskUsageUpdates() {
	if a.diskUsage == nil {
		return
	}
	needRender := false
	jobFinishedToast := false
	for {
		select {
		case ev := <-a.diskUsage.Events():
			switch ev.Kind {
			case diskusage.EventSubtreeIndexed:
				a.maybeScheduleIdleDiskSortBothPanels()
				needRender = true
			case diskusage.EventJobFinished:
				// Last-resort schedule after session completes (subtree events should suffice).
				a.maybeScheduleIdleDiskSortBothPanels()
				jobFinishedToast = true
				needRender = true
			}
		case <-a.diskUsage.Updates():
			needRender = true
		default:
			goto drained
		}
	}
drained:
	// Only user-initiated scans (startDiskUsageScanForPanel) arm the toast flag.
	// Selection-size background scans do not set it, so their EventJobFinished completions
	// never show the toast even when DiskUsageShown is true.
	if jobFinishedToast && a.diskUsageScanToastArmed {
		a.diskUsageScanToastArmed = false
		a.setTransientMessage("Disk usage scan finished", ui.MessageUrgencyInfo)
	}
	if !needRender {
		return
	}
	// While a scan is in flight, coalesce paint/sort work so bursty subtree completions do not
	// stall the PollEvent loop; job completion still flushes immediately.
	flushNow := jobFinishedToast || !a.diskUsageScanBusy()
	if flushNow {
		a.stopDiskUsageRedrawDebounce()
		a.resortPanelsDiskUsageSorted()
		a.refreshDeleteDialogSummary()
		if a.model.FindDialog.Open {
			a.model.FindDialog.InvalidateMarkedSelectionSizeLabel()
			a.renderFindDialogUpdate()
			return
		}
		if a.deleteDialogOpen() {
			a.renderDeleteDialogUpdate()
			return
		}
		a.render()
		return
	}
	a.scheduleDiskUsageRedrawDebounced()
}

func (a *App) resortPanelsDiskUsageSorted() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	vrLeft := a.panelViewportRows(ui.PrimaryPanel)
	vrRight := a.panelViewportRows(ui.SecondaryPanel)
	if a.model.Primary.Sort.DiskUsageIdleSizeSort && a.model.Primary.IdleDiskTotalsSort {
		a.model.Primary.RefreshDiskUsageOrdering(vrLeft, false)
	}
	if a.model.Secondary.Sort.DiskUsageIdleSizeSort && a.model.Secondary.IdleDiskTotalsSort {
		a.model.Secondary.RefreshDiskUsageOrdering(vrRight, false)
	}
}

func (a *App) setDiskUsageScanScope(origin string, childRoots []string) {
	if origin == "" {
		a.model.DiskUsageScanOrigin = ""
	} else {
		a.model.DiskUsageScanOrigin = filepath.Clean(origin)
	}
	if len(childRoots) == 0 {
		a.model.DiskUsageScanRoots = nil
		return
	}
	roots := make([]string, len(childRoots))
	for i, raw := range childRoots {
		roots[i] = filepath.Clean(raw)
	}
	a.model.DiskUsageScanRoots = roots
}

func (a *App) listingInDiskUsageScanScope(listingPath string) bool {
	return panel.ListingPathInDiskUsageScanScope(listingPath, a.model.DiskUsageScanOrigin, a.model.DiskUsageScanRoots)
}

func (a *App) diskIdleSortPanelEligible(p *panel.State) bool {
	// DiskUsageIdleSizeSort is the user-visible toggle (config + sort dialog).
	// Do not require DiskUsageIdleSortActivated here: it was only set after a successful apply,
	// which prevented startup/dialog-enable paths from ever scheduling idle disk ordering.
	// Scope check is intentionally omitted: ListingFullyDiskCached (checked by all callers)
	// is the correct gate — cached data is valid regardless of which scan populated it.
	return a.model.ViewMode == ui.ViewBrowser &&
		p.Sort.DiskUsageIdleSizeSort &&
		!p.IdleDiskTotalsSort
}

func (a *App) maybeScheduleIdleDiskSortBothPanels() {
	a.maybeScheduleIdleDiskSort(ui.PrimaryPanel)
	a.maybeScheduleIdleDiskSort(ui.SecondaryPanel)
}

func (a *App) maybeScheduleIdleDiskSort(panelID int) {
	p := a.panelByID(panelID)
	if !a.diskIdleSortPanelEligible(p) {
		return
	}
	if !p.ListingFullyDiskCached() {
		return
	}
	a.armIdleDiskSortTimer(panelID)
}

func (a *App) applyIdleDiskSort(panelID int, epoch uint64) {
	if panelID != ui.PrimaryPanel && panelID != ui.SecondaryPanel {
		return
	}
	ps := &a.diskIdleSort[panelID]
	if ps.epoch != epoch {
		return
	}
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	p := a.panelByID(panelID)
	if !a.diskIdleSortPanelEligible(p) || p.IdleDiskTotalsSort {
		return
	}
	if !p.ListingFullyDiskCached() {
		return
	}
	p.DiskUsageIdleSortActivated = true
	p.IdleDiskTotalsSort = true
	p.RefreshDiskUsageOrdering(a.panelViewportRows(panelID), true)
}

func (a *App) armIdleDiskSortTimer(panelID int) {
	if panelID != ui.PrimaryPanel && panelID != ui.SecondaryPanel {
		return
	}
	ps := &a.diskIdleSort[panelID]
	if ps.timer != nil {
		ps.timer.Stop()
		ps.timer = nil
	}
	p := a.panelByID(panelID)
	if !a.diskIdleSortPanelEligible(p) {
		return
	}
	if !p.ListingFullyDiskCached() {
		return
	}
	delayMS := a.config.DiskUsageIdleSortDelayMS
	if delayMS <= 0 {
		delayMS = 500
	}
	delay := time.Duration(delayMS) * time.Millisecond
	epochSnap := ps.epoch
	pid := panelID
	ps.timer = time.AfterFunc(delay, func() {
		ps.timer = nil
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(diskIdleSortPayload{PanelID: pid, Epoch: epochSnap}))
	})
}

func (a *App) invalidateIdleDiskSortPanel(panelID int) {
	if panelID != ui.PrimaryPanel && panelID != ui.SecondaryPanel {
		return
	}
	ps := &a.diskIdleSort[panelID]
	if ps.timer != nil {
		ps.timer.Stop()
		ps.timer = nil
	}
	ps.epoch++
}

func (a *App) invalidateIdleDiskSortBothPanels() {
	a.invalidateIdleDiskSortPanel(ui.PrimaryPanel)
	a.invalidateIdleDiskSortPanel(ui.SecondaryPanel)
}

func (a *App) deferDiskIdleSortOnUserActivity() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	if a.diskUsageScanBusy() {
		return
	}
	a.maybeScheduleIdleDiskSort(ui.PrimaryPanel)
	a.maybeScheduleIdleDiskSort(ui.SecondaryPanel)
}

func (a *App) startDiskUsageScan() {
	a.startDiskUsageScanForPanel(a.model.ActivePanel)
}

func (a *App) abortAllDiskUsageScans() {
	if a.diskUsage == nil {
		return
	}
	if !a.diskUsageScanBusy() {
		a.setTransientMessage("No disk usage scan in progress", ui.MessageUrgencyInfo)
		return
	}
	a.diskUsage.Abort()
	a.invalidateIdleDiskSortBothPanels()
	a.setTransientMessage("Disk usage scans aborted", ui.MessageUrgencyInfo)
	a.pollDiskUsageUpdates()
}

func (a *App) clearAllDiskUsageData() {
	if a.diskUsage == nil {
		return
	}
	a.stopDiskUsageRedrawDebounce()
	a.diskUsage.ClearCache()
	a.model.DiskUsageShown = false
	a.model.DiskUsagePanelID = ui.PrimaryPanel
	a.setDiskUsageScanScope("", nil)
	for _, panelID := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		p := a.panelByID(panelID)
		p.IdleDiskTotalsSort = false
	}
	a.invalidateIdleDiskSortBothPanels()
	vrLeft := a.panelViewportRows(ui.PrimaryPanel)
	vrRight := a.panelViewportRows(ui.SecondaryPanel)
	_ = a.model.Primary.Refresh(vrLeft)
	_ = a.model.Secondary.Refresh(vrRight)
	a.setTransientMessage("Disk usage data cleared", ui.MessageUrgencyInfo)
}

func (a *App) startDiskUsageScanForPanel(panelID int) {
	if a.diskUsage == nil || a.diskUsageIgnore == nil {
		return
	}
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	p := a.panelByID(panelID)
	if p.Path.IsRemote() {
		a.setTransientMessage("Disk usage is not available on remote panels", ui.MessageUrgencyWarn)
		return
	}
	p.IdleDiskTotalsSort = false
	a.invalidateIdleDiskSortPanel(panelID)

	childPaths := make([]string, 0, len(p.Entries))
	for _, entry := range p.Entries {
		childPaths = append(childPaths, filepath.Clean(entry.Path))
	}

	a.setDiskUsageScanScope(p.PathString(), childPaths)

	a.diskUsage.StartScanFromListing(childPaths, a.diskUsageIgnore, panelID, listingVolumeGateForScan(p, a.config.DiskUsageDescendIntoMountPoints))
	a.model.DiskUsageShown = true
	a.diskUsageScanToastArmed = true
	a.model.DiskUsagePanelID = panelID
	a.model.DiskUsage = a.diskUsage
	a.setTransientMessage("Disk usage scan started ("+filepath.Clean(p.PathString())+")", ui.MessageUrgencyInfo)
}

func (a *App) diskUsageScanBusy() bool {
	if a.model.DiskUsage == nil {
		return false
	}
	return a.model.DiskUsage.DiskScanBusy()
}

func (a *App) menuBarSpinnerBusy() bool {
	return a.diskUsageScanBusy() || a.jobState.HasUnfinishedWork() || a.hasRunningCommands()
}

func (a *App) stopSpinnerRedrawTimer() {
	if a.spinnerRedrawTimer == nil {
		return
	}
	a.spinnerRedrawTimer.Stop()
	a.spinnerRedrawTimer = nil
}

func (a *App) armSpinnerRedrawTimer() {
	if !a.menuBarSpinnerBusy() {
		a.stopSpinnerRedrawTimer()
		return
	}
	if a.spinnerRedrawTimer != nil {
		return
	}
	const delay = 90 * time.Millisecond
	a.spinnerRedrawTimer = time.AfterFunc(delay, func() {
		a.spinnerRedrawTimer = nil
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(spinnerTickPayload{}))
	})
}

func (a *App) stopDiskUsageRedrawDebounce() {
	if a.diskUsageRedrawTimer == nil {
		return
	}
	a.diskUsageRedrawTimer.Stop()
	a.diskUsageRedrawTimer = nil
}

func (a *App) scheduleDiskUsageRedrawDebounced() {
	if a.diskUsageRedrawTimer != nil {
		return
	}
	const debounce = 75 * time.Millisecond
	a.diskUsageRedrawTimer = time.AfterFunc(debounce, func() {
		a.diskUsageRedrawTimer = nil
		if a.diskUsage == nil {
			return
		}
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(diskUsageRedrawPayload{}))
	})
}

// handlePanelDirChanged reconciles disk-usage idle-sort for the given panel. It is
// invoked from App.reconcileAfterEvent for both panels every iteration of the Run loop.
// Idempotent: short-circuits when DiskUsageIdleSizeSort is off (the common case) or when
// appropriate. If the listing is fully disk-cached, disk-total ordering applies immediately;
// otherwise pending idle-sort debounce for this panel is invalidated on cwd change.
func (a *App) handlePanelDirChanged(panelID int) {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	p := a.panelByID(panelID)
	if !p.Sort.DiskUsageIdleSizeSort {
		return
	}
	cur := filepath.Clean(p.PathString())
	if a.diskIdleNavPath[panelID] != cur {
		a.invalidateIdleDiskSortPanel(panelID)
		a.diskIdleNavPath[panelID] = cur
	}
	if len(p.Entries) == 0 {
		return
	}
	if p.IdleDiskTotalsSort {
		return
	}
	if p.ListingFullyDiskCached() {
		p.DiskUsageIdleSortActivated = true
		p.IdleDiskTotalsSort = true
		p.RefreshDiskUsageOrdering(a.panelViewportRows(panelID), true)
		return
	}
}
