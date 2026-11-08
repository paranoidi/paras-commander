package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func (a *App) toastNeedsFullRender() bool {
	return strings.TrimSpace(a.model.Message) != a.lastFullRenderToast
}

func (a *App) recordFullRenderToast() {
	a.lastFullRenderToast = strings.TrimSpace(a.model.Message)
}

// paintOverlay is the one partial repaint of a dialog overlay over the unchanged screen: paint
// draws the dialog and returns the area it painted. It declines (false: the caller does a full
// render) when a toast needs the full render, the terminal is too small, or the area differs
// from the one on screen (a frame that shrank or moved, a dropdown that closed), so what the
// old area covered is repainted rather than left as a ghost.
func (a *App) paintOverlay(kind ui.OverlayKind, paint func(layout ui.Layout) dialog.PaintedArea) bool {
	if a.toastNeedsFullRender() {
		return false
	}
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	if layout.TooSmall {
		return false
	}
	if overlayAreaChanged(&a.overlays[kind], paint(layout)) {
		return false
	}
	ui.PaintTransientStatusMessage(a.screen, layout, a.model, a.styles)
	a.emitScreenAfterPartialPaint()
	return true
}

// overlayAreaChanged records area as the overlay's area on screen and reports whether it
// differs from the one recorded before.
func overlayAreaChanged(onScreen *dialog.PaintedArea, area dialog.PaintedArea) bool {
	prev := *onScreen
	*onScreen = area
	return prev != area
}

// paintFindDialogOverlay repaints only the find dialog (paintOverlay). It is allowed only while
// Find is the top input layer; a higher overlay (Group Select opened from Find, a message
// dialog, …) must go through a full render so that layer stays on top.
func (a *App) paintFindDialogOverlay() bool {
	if !a.model.FindDialog.Open || a.inputMode() != InputModeFindDialog {
		return false
	}
	return a.paintOverlay(ui.OverlayFind, func(layout ui.Layout) dialog.PaintedArea {
		return ui.PaintFindDialog(a.screen, layout, &a.model.FindDialog, a.styles, a.model.UseNerdfontIcons, a.model.PanelScrollbar, a.model.DiskUsage, a.model.PinnedItems, a.model.JobPathMarks)
	})
}

// paintFileDialogOverlay repaints only the file dialog (rename, mkdir, mass rename, delete, …;
// paintOverlay).
func (a *App) paintFileDialogOverlay() bool {
	if !a.model.FileDialog.Open {
		return false
	}
	return a.paintOverlay(ui.OverlayFile, func(layout ui.Layout) dialog.PaintedArea {
		if a.model.FileDialog.DialogType == dialog.FileDialogMassRename && a.model.FileDialog.MassRenamePhase == dialog.MassRenamePhaseMain {
			a.dialogCtrl.RecomputeMassRenamePreview()
		}
		return ui.PaintFileDialog(a.screen, layout, a.model.FileDialog, a.styles, a.model.UseNerdfontIcons, a.model.PanelScrollbar)
	})
}

// renderFindDialogUpdate repaints the find overlay when it is open; otherwise falls back to a full render.
func (a *App) renderFindDialogUpdate() {
	if a.findCtrl.Hidden() {
		return // Find is parked or behind the viewer it opened: nothing of it is on screen
	}
	if a.model.FindDialog.Open && a.paintFindDialogOverlay() {
		return
	}
	a.render()
}

// renderDeleteDialogUpdate repaints the delete overlay when it is open; otherwise falls back to a full render.
func (a *App) renderDeleteDialogUpdate() {
	if a.deleteDialogOpen() && a.paintFileDialogOverlay() {
		return
	}
	a.render()
}

// panelOnlyPaintBlocked reports whether an overlay (modal dialog or pulldown menu) is drawn on
// top of the panels. A panel-only partial paint would then paint over that overlay and leave it
// corrupted until something repaints it, so such frames must go through a full ui.Render, which
// redraws the overlay on top of the panels in the same frame.
func (a *App) panelOnlyPaintBlocked() bool {
	return a.model.ModalDialogOpen() || a.model.Menu.Open
}

// browserListNavPartialRenderEligible reports whether file-list navigation can repaint only the
// active column (inactive panel unchanged this frame).
func (a *App) browserListNavPartialRenderEligible() bool {
	if a.model.ViewMode != ui.ViewBrowser || a.model.ActiveSubFocus != ui.SubFocusFileList {
		return false
	}
	if a.panelOnlyPaintBlocked() {
		return false
	}
	if a.model.SyncFollowEnabled && !a.syncFollowNav.Held() {
		return false
	}
	if a.model.QuickViewEnabled && a.model.QuickViewDisplayActive() && !a.previewCtrl.QuickViewNavSkipReconcile() {
		return false
	}
	return true
}

// renderPanelAsyncResult repaints after an async result has been applied to panelID, preferring
// the cheaper single-panel partial repaint over a full-screen render when eligible — an async
// apply is not itself a user-driven navigation event, so this checks eligibility directly rather
// than going through browserListNavPartialRenderEligibleFor's per-action gate.
func (a *App) renderPanelAsyncResult(panelID int) {
	if a.browserListNavPartialRenderEligible() {
		a.renderBrowserListNavUpdate(panelID, false)
	} else {
		a.render()
	}
}

// carouselParentPaintPending reports whether panelID is showing a carousel whose parent-column
// snapshot has not caught up with the panel's current directory. The carousel's column geometry is
// measured from the parent listing when the parent split is fit-to-content, so painting in this
// window would lay the columns out against the previous directory's names and then visibly
// re-lay them out one frame later, when the parent snapshot lands.
func (a *App) carouselParentPaintPending(panelID int) bool {
	if panelID != ui.PrimaryPanel && panelID != ui.SecondaryPanel {
		return false
	}
	pan := a.panelByID(panelID)
	if pan == nil || !pan.CarouselMode || pan.Path.IsZero() {
		return false
	}
	if pan.Path.Parent().Equal(pan.Path) {
		return false // at the filesystem root there is no parent column to wait for
	}
	return !pan.CarouselParentCacheValid()
}

// renderAfterAsyncApply repaints for an applied async panel-navigation result, except while the
// panel's carousel parent column is still catching up (carouselParentPaintPending) — then the
// paint is deferred so the new directory is drawn once, with correct column widths, instead of
// once against the old parent's measurements and again a frame later. The deferred paint is
// always released: raceAsyncListingFetch posts a result for every dispatch, timeouts included, and
// the carouselSnapshotPayload arm renders on arrival whether or not the snapshot applied cleanly.
func (a *App) renderAfterAsyncApply(panelID int) {
	if a.holdBrowserPaintForCarouselParent(panelID) {
		return
	}
	a.renderPanelAsyncResult(panelID)
}

// holdBrowserPaintForCarouselParent defers a panel paint while the carousel parent-column
// snapshot is still catching up. Shared by async-apply and keyboard folder-change partial paints.
func (a *App) holdBrowserPaintForCarouselParent(panelID int) bool {
	if !a.carouselParentPaintPending(panelID) {
		return false
	}
	a.carouselPaintDefer[panelID].active = true
	a.armCarouselPaintDeferTimer(panelID)
	return true
}

// snapshotModelForBrowserPaint copies a.model under commandsMu and applies the pin-out /
// autohide / terminal-focus rules both list-nav and disk-usage partial paints need so they
// agree with a full render's drawBrowserView plan.
func (a *App) snapshotModelForBrowserPaint() ui.Model {
	a.commandsMu.RLock()
	model := a.model
	a.commandsMu.RUnlock()
	model.CursorNameHintPinOutPrimary = &a.model.Primary.CursorNameHintPinned
	model.CursorNameHintPinOutSecondary = &a.model.Secondary.CursorNameHintPinned
	model.HideInactivePanel = a.effectiveHideInactivePanel()
	// Match ui.Model.renderSubFocus: suppress panel/strip focus while the terminal owns input.
	if model.TerminalPanel.Visible && model.TerminalPanel.Focused {
		model.ActiveSubFocus = -1
	}
	return model
}

// renderBrowserListNavUpdate repaints panelID's file-list column and menu-bar permission tail
// without a full render. The other panel is repainted only when its match marks can have changed:
// after a listing change (cursorOnly false) whenever the marks are on, after a cursor-only move
// only when ui.PaintBrowserCursorMovePanelOnly finds a row that would carry a mark.
func (a *App) renderBrowserListNavUpdate(panelID int, cursorOnly bool) {
	if a.holdBrowserPaintForCarouselParent(panelID) {
		a.navPaintDeferred = false // the carousel timer paints later; nothing left for the loop to flush
		return
	}
	if a.toastNeedsFullRender() {
		a.render()
		return
	}
	a.previewCtrl.SyncCarouselChildPreviewCoalesceFlags()
	a.syncCursorNameHintNavCoalesceFlags()
	a.model.MenuBarPermission = a.menuBarPermissionText()
	a.model.MenuBarActivitySpinner = a.menuBarSpinnerVisible()
	w, h := a.screen.Size()
	if a.zoomLayoutChanged(w, h) {
		a.render()
		return
	}
	layout := a.layoutForTerminalSize(w, h)
	model := a.snapshotModelForBrowserPaint()
	paint := ui.PaintBrowserListNavPanelOnly
	if cursorOnly {
		paint = ui.PaintBrowserCursorMovePanelOnly
	}
	if layout.TooSmall || !paint(a.screen, layout, model, a.styles, panelID) {
		a.render()
		return
	}
	ui.DrawMenuBarPermissionTailOnly(a.screen, layout, model, a.styles)
	ui.PaintTransientStatusMessage(a.screen, layout, model, a.styles)
	a.emitScreenAfterPartialPaint()
	if a.diskUsageScanBusy() {
		a.disk.deferPoll.Store(true)
	}
	a.markNavPainted()
}

// markNavPainted records that the frame is current: clears a deferred nav paint and stamps the end
// of the paint (after the blocking Show) so a slow terminal's write time is not counted as staleness.
func (a *App) markNavPainted() {
	a.navPaintDeferred = false
	a.lastNavPaintAt = time.Now()
}

// shouldDeferNavPaint reports whether a cursor-only nav paint can be skipped because more events
// are already queued and the last paint is recent. Only inside Run: direct handleKey callers
// (tests) and idle queues keep painting synchronously.
func (a *App) shouldDeferNavPaint() bool {
	return a.runLoopActive && a.screen.HasPendingEvent() &&
		time.Since(a.lastNavPaintAt) < config.NavPaintMaxStale
}

// flushDeferredNavPaint paints a skipped nav frame once the queue drains or the frame went stale.
// The deferred frame is cursor-only, so it takes the narrow partial paint; both paint paths clear
// navPaintDeferred (markNavPainted), so an async apply landing in between supersedes it.
func (a *App) flushDeferredNavPaint() {
	if a.navPaintDeferred && (!a.screen.HasPendingEvent() || time.Since(a.lastNavPaintAt) >= config.NavPaintMaxStale) {
		if a.browserListNavPartialRenderEligible() {
			a.renderBrowserListNavUpdate(a.navPaintDeferredPanel, true)
		} else {
			a.render()
		}
	}
}

// paintDiskUsageBrowserUpdate repaints disk-usage scan-scope panels and the menu-bar spinner
// without a full twin-panel render. Returns false when ui.Render is required instead.
func (a *App) paintDiskUsageBrowserUpdate() bool {
	if a.model.ViewMode != ui.ViewBrowser || a.panelOnlyPaintBlocked() {
		return false
	}
	if a.toastNeedsFullRender() {
		return false
	}
	a.model.MenuBarActivitySpinner = a.menuBarSpinnerVisible()
	w, h := a.screen.Size()
	if a.zoomLayoutChanged(w, h) {
		return false
	}
	layout := a.layoutForTerminalSize(w, h)
	if layout.TooSmall {
		return false
	}
	model := a.snapshotModelForBrowserPaint()
	if !ui.PaintDiskUsageBrowserPanelsOnly(a.screen, layout, model, a.styles) {
		return false
	}
	ui.PaintTransientStatusMessage(a.screen, layout, model, a.styles)
	a.emitScreenAfterPartialPaint()
	return true
}

func (a *App) deleteDialogOpen() bool {
	return a.model.FileDialog.Open && a.model.FileDialog.DialogType == dialog.FileDialogDelete
}

func (a *App) render() {
	a.stopDiskUsageRedrawDebounce()
	a.previewCtrl.SyncCarouselChildPreviewCoalesceFlags()
	a.syncCursorNameHintNavCoalesceFlags()
	a.model.PanelZoomActivePercent = a.config.UI.Zoom.ActivePercent
	a.model.PanelZoomInactivePercent = a.config.UI.Zoom.InactivePercent
	a.model.NarrowPanelsNameOnly = a.config.UI.NarrowPanelsNameOnly
	a.model.OtherPanelMatchMarks = a.config.UI.OtherPanelMatchMarks
	sb, _ := uiscrollbar.ParseStyle(a.config.UI.Scroll.Scrollbar)
	a.model.PanelScrollbar = uiscrollbar.EffectiveStyle(sb)
	a.model.PanelScrollbarInactive = a.config.UI.Scroll.ScrollbarInactive
	// Reconcile derived model (latched panel sync, disk-usage idle-sort hooks, etc.)
	// before painting so the frame matches post-mutation state. The Run loop also calls
	// reconcileAfterEvent() after each event; without this ordering, render() would run
	// first on key handling and latched sync would appear one selection behind until the
	// next redraw.
	a.reconcileAfterEvent()
	a.model.MenuBarPermission = a.menuBarPermissionText()
	a.model.MenuBarJobsAttention = a.menuBarJobsAttentionText()
	a.model.StatusCommands = a.statusCommandSegments()
	a.refreshMenuBarJobsStrip()
	a.model.MenuBarActivitySpinner = a.menuBarSpinnerVisible()
	a.model.FooterKeys = a.activeFooterKeys()
	a.model.JobsTransferRateLimitBPS = a.jobState.TransferRateLimit()
	a.model.JobsFilterCaseInsensitive = a.config.Filter.CaseInsensitive
	if a.model.ViewMode == ui.ViewJobs {
		a.jobsCtrl.RefreshOpsWindow()
	}
	a.model.DiskUsageDescendIntoMountPoints = a.config.DiskUsage.DescendIntoMountPoints
	a.model.DiskUsageGoduIgnore = a.disk.ignore
	a.previewCtrl.SnapshotPreviewDrawStates()
	previewOpen := a.model.FilePreviewDraw.Open || a.model.QuickViewDisplayActive()
	if a.model.ViewMode == ui.ViewFilePreview {
		a.previewCtrl.ClampFullscreenFilePreviewScroll()
	}
	w, h := a.screen.Size()
	a.model.SplitOrientation = a.effectivePaneSplitOrientation()
	a.model.PanelZoomEnabled = a.effectiveZoomActivePanelLayout(w, h, previewOpen)
	a.applyCarouselPanelZoomPercents(w)
	if a.model.ViewMode == ui.ViewCommands {
		a.commandsCtrl.EnsureSelectionVisible() // rows appended while open shift the display
		a.commandsMu.RLock()
		a.model.CommandsDisplay = append([]ui.CommandRunEntry(nil), a.model.CommandsList...)
		a.commandsMu.RUnlock()
	} else {
		a.model.CommandsDisplay = nil
	}
	if a.model.FileDialog.Open && a.model.FileDialog.DialogType == dialog.FileDialogMassRename && a.model.FileDialog.MassRenamePhase == dialog.MassRenamePhaseMain {
		a.dialogCtrl.RecomputeMassRenamePreview()
	}
	// Copy a.model under commandsMu: passing it by value into ui.Render otherwise reads
	// every field (including CarouselFilePreview/FilePreview/FullscreenFilePreview, which
	// background preview goroutines mutate under this same lock) without synchronization.
	a.refreshPointerHovers()
	a.commandsMu.RLock()
	modelSnapshot := a.model
	a.commandsMu.RUnlock()
	modelSnapshot.CursorNameHintPinOutPrimary = &a.model.Primary.CursorNameHintPinned
	modelSnapshot.CursorNameHintPinOutSecondary = &a.model.Secondary.CursorNameHintPinned
	modelSnapshot.HideInactivePanel = a.effectiveHideInactivePanel()
	a.overlays = ui.Render(a.screen, modelSnapshot, a.styles)
	a.syncTerminalPanelCursor()
	a.emitScreenAfterFullRender()
	a.armSpinnerRedrawTimer()
	a.markNavPainted()
}

// emitScreenAfterFullRender flushes the terminal after ui.Render. When ScreenRenderHashCache is on,
// identical consecutive frames skip Show() to reduce redundant terminal traffic.
// Terminal images are locked before Show and emitted after; a changed placement forces Show.
func (a *App) emitScreenAfterFullRender() {
	a.recordFullRenderToast()
	plan := previewpanel.TakeFrameImage()
	if a.imageOverlaySuppressed() {
		plan = nil
	}
	force := a.reconcileImageBeforeShow(plan)
	if a.config.UI.ScreenRenderHashCache {
		h := ui.HashScreenLogical(a.screen)
		if !force && h == a.lastScreenContentHash && a.pendingCursor == a.lastFlushedCursor {
			return
		}
		a.lastScreenContentHash = h
		a.lastFlushedCursor = a.pendingCursor
	}
	a.screen.Show()
	a.emitImageAfterShow()
}

// emitScreenAfterPartialPaint runs after targeted SetContent calls (e.g. menu-bar spinner) so
// the hash cache stays aligned with what the terminal displays.
// A nil image plan (typical for partial paints) leaves the locked region alone.
func (a *App) emitScreenAfterPartialPaint() {
	if plan := previewpanel.TakeFrameImage(); plan != nil {
		if a.imageOverlaySuppressed() {
			plan = nil
		}
		_ = a.reconcileImageBeforeShow(plan)
	}
	a.screen.Show()
	a.emitImageAfterShow()
	if a.config.UI.ScreenRenderHashCache {
		a.lastScreenContentHash = ui.HashScreenLogical(a.screen)
		a.lastFlushedCursor = a.pendingCursor
	}
}

// refreshMenuBarJobsStrip rebuilds Model.MenuBarJobs from job state and stamps the light-bar head.
func (a *App) refreshMenuBarJobsStrip() {
	a.model.MenuBarJobs = a.jobsCtrl.MenuBarStripSnapshot()
	if a.menuBarSpeedStale {
		a.menuBarSpeed, a.menuBarSpeedStale = a.model.MenuBarJobs.Speed, false
	}
	a.model.MenuBarJobs.Speed = a.menuBarSpeed
	if a.devDeleteBarDemoActive() {
		frac := float64(time.Since(a.devDeleteBarDemoStart)) / float64(devDeleteBarDemoFill)
		if frac > 1 {
			frac = 1
		}
		a.model.MenuBarJobs = ui.MenuBarJobsStrip{
			HasProgress:  true,
			Deleting:     true,
			ProgressFrac: frac,
		}
	}
	if a.devMoveBarDemoActive() {
		a.model.MenuBarJobs = devMoveBarDemoStrip(time.Since(a.devMoveBarDemoStart))
	}
	a.model.MenuBarJobs.LightbarHead = a.lightbarHead
}

const (
	devDeleteBarDemoFill  = 4 * time.Second
	devDeleteBarDemoTotal = 10 * time.Second
)

func (a *App) devDeleteBarDemoActive() bool {
	return !a.devDeleteBarDemoStart.IsZero() && time.Since(a.devDeleteBarDemoStart) < devDeleteBarDemoTotal
}

const (
	// devMoveBarDemoTotal is how long Dev → Move bar demo runs; the bar fills over all of it.
	devMoveBarDemoTotal = 15 * time.Second
	// devMoveBarDemoRemoveEvery and devMoveBarDemoRemoveFor time the simulated source removals:
	// one delete-coloured phase of RemoveFor at the end of every RemoveEvery.
	devMoveBarDemoRemoveEvery = 3 * time.Second
	devMoveBarDemoRemoveFor   = 500 * time.Millisecond
)

func (a *App) devMoveBarDemoActive() bool {
	return !a.devMoveBarDemoStart.IsZero() && time.Since(a.devMoveBarDemoStart) < devMoveBarDemoTotal
}

// devMoveBarDemoStrip is the menu-bar strip of a simulated move elapsed into Dev → Move bar demo:
// one running job filling over devMoveBarDemoTotal, switching to the removal colours (fill, speed
// slot and groups unchanged) while a source removal is simulated.
func devMoveBarDemoStrip(elapsed time.Duration) ui.MenuBarJobsStrip {
	return ui.MenuBarJobsStrip{
		Groups:       []ui.MenuBarJobGroup{{Status: string(jobs.StatusRunning), Count: 1}},
		HasProgress:  true,
		ProgressFrac: min(float64(elapsed)/float64(devMoveBarDemoTotal), 1),
		Removing:     elapsed%devMoveBarDemoRemoveEvery >= devMoveBarDemoRemoveEvery-devMoveBarDemoRemoveFor,
		SpeedSlot:    true,
		Speed:        "120MB/s",
	}
}

// paintMenuBarJobsStripOnly updates Model.MenuBarJobs and repaints only the menu-bar jobs gap
// (queue + progress between labels and permission tail). Caller sets lastJobBatchMenuBarStripOnly.
// Returns painted (whether the gap was repainted) and lightbarExited (see ui.DrawMenuBarJobsGap) —
// the spinner tick handler uses the latter to reset the light bar's frame counter.
func (a *App) paintMenuBarJobsStripOnly() (painted, lightbarExited bool) {
	if !a.model.MenuBarLayoutReserved() {
		return false, false
	}
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	if layout.TooSmall {
		return false, false
	}
	a.refreshMenuBarJobsStrip()
	menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
	painted, lightbarExited = ui.DrawMenuBarJobsGapOnly(a.screen, layout, a.model, menus, a.styles)
	if !painted {
		return false, false
	}
	a.emitScreenAfterPartialPaint()
	return true, lightbarExited
}

func (a *App) menuBarJobsAttentionText() string {
	n := a.jobState.JobsWaitingDecision()
	if n <= 0 {
		return ""
	}
	word := "jobs"
	if n == 1 {
		word = "job"
	}
	return fmt.Sprintf("󰋗 %d %s waiting", n, word)
}

// statusCommandWidth returns the columns to reserve at the top-left of the menu bar for
// the status-command segments this frame (0 when none is shown or has output).
func (a *App) statusCommandWidth() int {
	return ui.StatusCommandWidth(a.statusCommandSegments())
}

func (a *App) menuBarPermissionText() string {
	if ui.IsAuxiliaryView(a.model.ViewMode) {
		return ""
	}
	entry, ok := a.activePanel().CurrentEntry()
	if !ok {
		return ""
	}
	return localfs.UnixModeString(entry.Mode)
}

// layoutForTerminalSizePreview computes the browser layout. When filePreviewOpen is true, the
// twin-column split treats panel zoom as off (same rule as when preview is actually open) so
// callers can size subprocess output (e.g. bat --terminal-width) before preview state is toggled.
func (a *App) layoutForTerminalSizePreview(width, height int, filePreviewOpen bool) ui.Layout {
	return ui.CalculateLayoutWithOrientation(width, height, a.model.MenuBarLayoutReserved(), a.panelPaneSplit(width, filePreviewOpen), a.effectivePaneSplitOrientation(), a.terminalLayoutRows(), a.statusCommandWidth())
}

// applyCarouselPanelZoomPercents widens the active column when carousel view is on so three panes fit.
func (a *App) applyCarouselPanelZoomPercents(totalWidth int) {
	if !a.activePanel().CarouselMode || !a.model.PanelZoomEnabled {
		return
	}
	minPct := panelcarousel.MinActiveWidthPercent(totalWidth, a.model.CarouselLayout)
	if a.model.PanelZoomActivePercent < minPct {
		a.model.PanelZoomActivePercent = minPct
		a.model.PanelZoomInactivePercent = 100 - minPct
	}
}

// panelPaneSplit is the browser split for width cells (<= 0: the terminal width), with
// every reason to hide the inactive panel applied.
func (a *App) panelPaneSplit(width int, filePreviewOpen bool) ui.PanelPaneSplit {
	split := a.visiblePanelPaneSplit(width, filePreviewOpen)
	hide := a.model.HideInactivePanel || a.carouselAutohideInactivePanel() || (!filePreviewOpen && a.inactivePanelTooNarrow(split, width))
	split.HideInactivePanel = ui.LayoutHideInactivePanel(a.model.ViewMode, hide)
	return split
}

// visiblePanelPaneSplit is the split with both panels on screen, the shares
// inactivePanelTooNarrow measures: its result feeds the hide flag but never these widths.
func (a *App) visiblePanelPaneSplit(width int, filePreviewOpen bool) ui.PanelPaneSplit {
	w, h := a.screen.Size()
	if width <= 0 {
		width = w
	}
	zoom := a.effectiveZoomActivePanelLayout(width, h, filePreviewOpen)
	activePct := a.model.PanelZoomActivePercent
	inactivePct := a.model.PanelZoomInactivePercent
	if activePct <= 0 || inactivePct <= 0 {
		activePct = a.config.UI.Zoom.ActivePercent
		inactivePct = a.config.UI.Zoom.InactivePercent
	}
	if zoom && a.activePanel().CarouselMode {
		minPct := panelcarousel.MinActiveWidthPercent(width, a.model.CarouselLayout)
		if activePct < minPct {
			activePct = minPct
			inactivePct = 100 - activePct
		}
	}
	return ui.PanelPaneSplit{
		Zoom:            ui.PanelZoomSplitsColumns(a.model.ViewMode, zoom),
		ActivePanel:     a.model.ActivePanel,
		ActivePercent:   activePct,
		InactivePercent: inactivePct,
		SwapPanes:       a.model.ViewMode == ui.ViewBrowser && a.model.EffectiveSwapPanes(),
	}
}

// carouselAutohideWanted reports whether config and state call for hiding the inactive twin
// panel because the active panel is in carousel mode, ignoring the session override. It does
// not engage while sync-follow or quick view is on, since both need both panels visible.
func (a *App) carouselAutohideWanted() bool {
	return a.config.Carousel.AutohideInactivePanel &&
		a.activePanel().CarouselMode &&
		!a.model.SyncFollowEnabled &&
		!a.model.QuickViewEnabled
}

// carouselAutohideInactivePanel reports whether the inactive twin panel is hidden by carousel
// autohide (wanted and not overridden by the hide-inactive-panel toggle).
func (a *App) carouselAutohideInactivePanel() bool {
	return a.carouselAutohideWanted() && !a.carouselAutohideOverride
}

// inactivePanelTooNarrow reports whether the inactive panel could not list legible names even
// with nothing but names, given its share of split. Stacked panes take the full width, and sync
// follow and quick view need both panels, as for carousel autohide.
func (a *App) inactivePanelTooNarrow(split ui.PanelPaneSplit, width int) bool {
	if a.model.ViewMode != ui.ViewBrowser || a.effectivePaneSplitOrientation() != ui.SplitHorizontal ||
		a.model.SyncFollowEnabled || a.model.QuickViewEnabled {
		return false
	}
	if width <= 0 {
		width, _ = a.screen.Size()
	}
	return ui.PanelNameOnlyWidth(ui.InactivePaneWidth(width, split), *a.inactivePanel(), a.model.UseNerdfontIcons) < config.MinPanelNameWidth
}

// narrowAutohideInactivePanel reports whether the inactive panel is hidden for being too narrow
// at the current terminal size.
func (a *App) narrowAutohideInactivePanel() bool {
	filePreviewOpen := a.previewCtrl.FilePreviewOpen() || a.model.QuickViewDisplayActive()
	return !filePreviewOpen && a.inactivePanelTooNarrow(a.visiblePanelPaneSplit(0, false), 0)
}

// autohideInactivePanel reports whether the inactive panel is hidden by anything other than the
// hide-inactive-panel toggle.
func (a *App) autohideInactivePanel() bool {
	return a.carouselAutohideInactivePanel() || a.narrowAutohideInactivePanel()
}

// effectiveHideInactivePanel is Model.HideInactivePanel as a render pass applies it.
func (a *App) effectiveHideInactivePanel() bool {
	return a.model.HideInactivePanel || a.autohideInactivePanel()
}

// setCarouselAutohideOverride writes carouselAutohideOverride and republishes the ordering flag.
func (a *App) setCarouselAutohideOverride(v bool) {
	a.carouselAutohideOverride = v
	a.syncCarouselQuickViewOrdering()
}

// syncCarouselQuickViewOrdering publishes the config and override half of the forced quick-view
// order to Model.CarouselQuickViewOrdering. Call it after any change to either input.
func (a *App) syncCarouselQuickViewOrdering() {
	a.model.CarouselQuickViewOrdering = a.config.Carousel.AutohideInactivePanel && !a.carouselAutohideOverride
}

func (a *App) layoutForTerminalSize(width, height int) ui.Layout {
	return a.layoutForTerminalSizePreview(width, height, a.previewCtrl.FilePreviewOpen() || a.model.QuickViewDisplayActive())
}

// effectiveZoomActivePanel returns the saved zoom preference plus optional session-only override
// (Alt+Z). It does not consider terminal size, name width or file preview; see effectiveZoomActivePanelLayout.
func (a *App) effectiveZoomActivePanel() bool {
	if a.zoomActivePanelOverride != nil {
		return *a.zoomActivePanelOverride
	}
	return a.config.UI.Zoom.ActivePanel
}

// zoomActivePanelSuppressedByNameWidth reports whether the active panel's file-name column at an
// even 50/50 split of a width-cell terminal is at least [ui.zoom].disabled_above_name_width or
// every visible name already fits that column (zoom only helps cut-off names). The
// even split (width/2, whichever panel is active) is measured, never the current zoomed width, so
// the decision cannot oscillate with its own result nor flip when Tab swaps panels on odd widths.
// Icons, git strip, file columns and meta columns all count against the name column.
func (a *App) zoomActivePanelSuppressedByNameWidth(width int) bool {
	n := a.config.UI.Zoom.DisabledAboveNameWidth
	if n <= 0 {
		return false
	}
	nameW := ui.PanelNameColumnWidth(width/2, *a.activePanel(), a.model.UseNerdfontIcons,
		a.model.NarrowPanelsNameOnly, a.model.MetaResults[a.model.ActivePanel])
	// Zoom only helps when names are cut off; if they all fit at the even split, keep it.
	return nameW >= n || ui.PanelNamesFit(a.activePanel(), nameW)
}

func (a *App) zoomActivePanelSuppressedByTerminalHeight(height int) bool {
	n := a.config.UI.Zoom.DisabledAboveHeight
	return n > 0 && height >= n
}

// effectiveZoomActivePanelLayout is the zoom flag used for layout and Model.PanelZoomEnabled:
// preference from effectiveZoomActivePanel, disabled while preview/quick view owns the split,
// disabled when the active panel's file-name column at an even split is wide enough or holds
// every visible name (side by side) or the terminal is tall enough (stacked) per the [ui.zoom] gates (> 0).
// Carousel view on the active panel always enables zoom (ignores saved preference, runtime override, and size gates).
func (a *App) effectiveZoomActivePanelLayout(width, height int, filePreviewOpen bool) bool {
	if filePreviewOpen || ui.LayoutHideInactivePanel(a.model.ViewMode, a.model.HideInactivePanel) {
		return false
	}
	if a.activePanel().CarouselMode {
		return true
	}
	if !a.effectiveZoomActivePanel() {
		return false // preference off: skip the gates (the name-width gate formats meta columns)
	}
	// A stacked split does not change name width, so only the height gate applies there.
	if a.effectivePaneSplitOrientation() == ui.SplitVertical {
		return !a.zoomActivePanelSuppressedByTerminalHeight(height)
	}
	return !a.zoomActivePanelSuppressedByNameWidth(width)
}

// zoomLayoutChanged reports whether the zoom decision for the current terminal size differs from
// the one the last full render stored in Model.PanelZoomEnabled. The decision depends on content
// (meta, git and file columns), so it can change without a resize; a partial panel
// paint over a stale split would leave the other panel at the wrong rect and must fall back to
// a full render.
func (a *App) zoomLayoutChanged(w, h int) bool {
	return a.effectiveZoomActivePanelLayout(w, h, a.previewCtrl.FilePreviewOpen() || a.model.QuickViewDisplayActive()) != a.model.PanelZoomEnabled
}

// toggleRuntimeZoomActivePanel flips the zoom split the user currently sees (saved
// [ui.zoom].active_panel plus optional runtime-only override). When the flipped state
// matches the saved config value, the override is cleared so nil always means "follow saved".
func (a *App) toggleRuntimeZoomActivePanel() {
	effective := a.effectiveZoomActivePanel()
	next := !effective
	saved := a.config.UI.Zoom.ActivePanel
	if next == saved {
		a.zoomActivePanelOverride = nil
	} else {
		v := next
		a.zoomActivePanelOverride = &v
	}
	if next {
		a.setTransientMessage("Panel zoom: on", ui.MessageUrgencyInfo)
	} else {
		a.setTransientMessage("Panel zoom: off", ui.MessageUrgencyInfo)
	}
}
