package app

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// paintFindDialogOverlay repaints only the find dialog without redrawing panels or the footer.
func (a *App) paintFindDialogOverlay() bool {
	if !a.model.FindDialog.Open {
		return false
	}
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	if layout.TooSmall {
		return false
	}
	ui.PaintFindDialog(a.screen, layout, &a.model.FindDialog, a.styles, a.model.ShowFileIcons, a.model.DiskUsage, a.config.DiskUsage.DescendIntoMountPoints, a.diskUsageIgnore)
	ui.PaintTransientStatusMessage(a.screen, layout, a.model.Message, a.model.MessageUrgency, a.styles)
	a.emitScreenAfterPartialPaint()
	return true
}

// paintFileDialogOverlay repaints only the file dialog rect (rename, mkdir, copy/move,
// mass rename, delete, …) on top of the unchanged panel buffer, without redrawing panels
// or the footer. Returns false when the dialog is closed or the terminal is too small, so
// callers fall back to a full render.
func (a *App) paintFileDialogOverlay() bool {
	if !a.model.FileDialog.Open {
		return false
	}
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	if layout.TooSmall {
		return false
	}
	if a.model.FileDialog.DialogType == dialog.FileDialogMassRename {
		a.recomputeMassRenamePreview()
	}
	ui.PaintFileDialog(a.screen, layout, a.model.FileDialog, a.styles, a.model.ShowFileIcons)
	ui.PaintTransientStatusMessage(a.screen, layout, a.model.Message, a.model.MessageUrgency, a.styles)
	a.emitScreenAfterPartialPaint()
	return true
}

// renderFindDialogUpdate repaints the find overlay when it is open; otherwise falls back to a full render.
func (a *App) renderFindDialogUpdate() {
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

// browserListNavPartialRenderEligible reports whether file-list navigation can repaint only the
// active column (inactive panel unchanged this frame).
func (a *App) browserListNavPartialRenderEligible() bool {
	if a.model.ViewMode != ui.ViewBrowser || a.model.ActiveSubFocus != ui.SubFocusFileList {
		return false
	}
	if a.model.SyncFollowEnabled && !a.syncFollowNavSkipReconcile.Load() {
		return false
	}
	if a.model.QuickViewEnabled && a.model.QuickViewDisplayActive() && !a.quickViewNavSkipReconcile.Load() {
		return false
	}
	return true
}

// renderBrowserListNavUpdate repaints the active file-list column and menu-bar permission tail
// without redrawing the inactive panel (avoids disk-usage row work on the other column during scans).
func (a *App) renderBrowserListNavUpdate() {
	a.syncCarouselChildPreviewCoalesceFlags()
	a.syncCursorNameHintNavCoalesceFlags()
	a.model.MenuBarPermission = a.menuBarPermissionText()
	a.model.MenuBarActivitySpinner = a.menuBarSpinnerBusy()
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	model := a.model
	model.CursorNameHintPinOutPrimary = &a.model.Primary.CursorNameHintPinned
	model.CursorNameHintPinOutSecondary = &a.model.Secondary.CursorNameHintPinned
	if layout.TooSmall || !ui.PaintBrowserListNavPanelOnly(a.screen, layout, model, a.styles, a.model.ActivePanel) {
		a.render()
		return
	}
	ui.DrawMenuBarPermissionTailOnly(a.screen, layout, a.model, a.styles)
	ui.PaintTransientStatusMessage(a.screen, layout, a.model.Message, a.model.MessageUrgency, a.styles)
	a.emitScreenAfterPartialPaint()
	if a.diskUsageScanBusy() {
		a.deferDiskUsagePoll.Store(true)
	}
}

// paintDiskUsageBrowserUpdate repaints disk-usage scan-scope panels and the menu-bar spinner
// without a full twin-panel render. Returns false when ui.Render is required instead.
func (a *App) paintDiskUsageBrowserUpdate() bool {
	if a.model.ViewMode != ui.ViewBrowser {
		return false
	}
	a.model.MenuBarActivitySpinner = a.menuBarSpinnerBusy()
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	if layout.TooSmall {
		return false
	}
	model := a.model
	model.CursorNameHintPinOutPrimary = &a.model.Primary.CursorNameHintPinned
	model.CursorNameHintPinOutSecondary = &a.model.Secondary.CursorNameHintPinned
	if !ui.PaintDiskUsageBrowserPanelsOnly(a.screen, layout, model, a.styles) {
		return false
	}
	a.emitScreenAfterPartialPaint()
	return true
}

func (a *App) deleteDialogOpen() bool {
	return a.model.FileDialog.Open && a.model.FileDialog.DialogType == dialog.FileDialogDelete
}

func (a *App) render() {
	a.stopDiskUsageRedrawDebounce()
	a.syncCarouselChildPreviewCoalesceFlags()
	a.syncCursorNameHintNavCoalesceFlags()
	a.model.PanelZoomActivePercent = a.config.UI.Zoom.ActivePercent
	a.model.PanelZoomInactivePercent = a.config.UI.Zoom.InactivePercent
	a.model.ShrunkenShowsNameOnly = a.config.UI.ShrunkenShowsNameOnly
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
	a.model.MenuBarJobs = a.menuBarJobsStripSnapshot()
	a.model.MenuBarActivitySpinner = a.menuBarSpinnerBusy()
	a.model.FooterKeys = a.activeFooterKeys()
	a.model.DiskUsageDescendIntoMountPoints = a.config.DiskUsage.DescendIntoMountPoints
	a.model.DiskUsageGoduIgnore = a.diskUsageIgnore
	a.snapshotPreviewDrawStates()
	previewOpen := a.model.FilePreviewDraw.Open || a.model.QuickViewDisplayActive()
	if a.model.ViewMode == ui.ViewFilePreview {
		a.clampFullscreenFilePreviewScroll()
	}
	w, h := a.screen.Size()
	a.model.SplitOrientation = a.effectivePaneSplitOrientation()
	a.model.PanelZoomEnabled = a.effectiveZoomActivePanelLayout(w, h, previewOpen)
	a.applyCarouselPanelZoomPercents(w)
	if a.model.ViewMode == ui.ViewCommands {
		a.commandsMu.RLock()
		a.model.CommandsDisplay = append([]ui.CommandRunEntry(nil), a.model.CommandsList...)
		a.commandsMu.RUnlock()
	} else {
		a.model.CommandsDisplay = nil
	}
	if a.model.FileDialog.Open && a.model.FileDialog.DialogType == dialog.FileDialogMassRename {
		a.recomputeMassRenamePreview()
	}
	// Copy a.model under commandsMu: passing it by value into ui.Render otherwise reads
	// every field (including CarouselFilePreview/FilePreview/FullscreenFilePreview, which
	// background preview goroutines mutate under this same lock) without synchronization.
	a.commandsMu.RLock()
	modelSnapshot := a.model
	a.commandsMu.RUnlock()
	modelSnapshot.CursorNameHintPinOutPrimary = &a.model.Primary.CursorNameHintPinned
	modelSnapshot.CursorNameHintPinOutSecondary = &a.model.Secondary.CursorNameHintPinned
	ui.Render(a.screen, modelSnapshot, a.styles)
	a.syncTerminalPanelCursor()
	a.emitScreenAfterFullRender()
	a.armSpinnerRedrawTimer()
}

// emitScreenAfterFullRender flushes the terminal after ui.Render. When ScreenRenderHashCache is on,
// identical consecutive frames skip Show() to reduce redundant terminal traffic.
func (a *App) emitScreenAfterFullRender() {
	if !a.config.UI.ScreenRenderHashCache {
		a.screen.Show()
		return
	}
	h := ui.HashScreenLogical(a.screen)
	if h == a.lastScreenContentHash && a.pendingCursor == a.lastFlushedCursor {
		return
	}
	a.lastScreenContentHash = h
	a.lastFlushedCursor = a.pendingCursor
	a.screen.Show()
}

// emitScreenAfterPartialPaint runs after targeted SetContent calls (e.g. menu-bar spinner) so
// the hash cache stays aligned with what the terminal displays.
func (a *App) emitScreenAfterPartialPaint() {
	a.screen.Show()
	if a.config.UI.ScreenRenderHashCache {
		a.lastScreenContentHash = ui.HashScreenLogical(a.screen)
		a.lastFlushedCursor = a.pendingCursor
	}
}

// paintMenuBarJobsStripOnly updates Model.MenuBarJobs and repaints only the menu-bar jobs gap
// (queue + progress between labels and permission tail). Caller sets lastJobBatchMenuBarStripOnly.
func (a *App) paintMenuBarJobsStripOnly() bool {
	if !a.model.MenuBarLayoutReserved() {
		return false
	}
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	if layout.TooSmall {
		return false
	}
	a.model.MenuBarJobs = a.menuBarJobsStripSnapshot()
	menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
	if !ui.DrawMenuBarJobsGapOnly(a.screen, layout, a.model, menus, a.styles) {
		return false
	}
	a.emitScreenAfterPartialPaint()
	return true
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
	return ui.CalculateLayoutWithOrientation(width, height, a.model.MenuBarLayoutReserved(), a.panelPaneSplit(width, filePreviewOpen), a.effectivePaneSplitOrientation(), a.terminalLayoutRows())
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

func (a *App) panelPaneSplit(width int, filePreviewOpen bool) ui.PanelPaneSplit {
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
		Zoom:              ui.PanelZoomSplitsColumns(a.model.ViewMode, zoom),
		ActivePanel:       a.model.ActivePanel,
		ActivePercent:     activePct,
		InactivePercent:   inactivePct,
		HideInactivePanel: ui.LayoutHideInactivePanel(a.model.ViewMode, a.model.HideInactivePanel),
	}
}

func (a *App) layoutForTerminalSize(width, height int) ui.Layout {
	return a.layoutForTerminalSizePreview(width, height, a.filePreviewOpen() || a.model.QuickViewDisplayActive())
}

// effectiveZoomActivePanel returns the saved zoom preference plus optional session-only override
// (Alt+Z). It does not consider terminal width or file preview; see effectiveZoomActivePanelLayout.
func (a *App) effectiveZoomActivePanel() bool {
	if a.zoomActivePanelOverride != nil {
		return *a.zoomActivePanelOverride
	}
	return a.config.UI.Zoom.ActivePanel
}

func (a *App) zoomActivePanelSuppressedByTerminalWidth(width int) bool {
	n := a.config.UI.Zoom.DisabledAboveWidth
	return n > 0 && width >= n
}

func (a *App) zoomActivePanelSuppressedByTerminalHeight(height int) bool {
	n := a.config.UI.Zoom.DisabledAboveHeight
	return n > 0 && height >= n
}

// effectiveZoomActivePanelLayout is the zoom flag used for layout and Model.PanelZoomEnabled:
// preference from effectiveZoomActivePanel, disabled while preview/quick view owns the split,
// and disabled on wide/tall terminals when the corresponding [ui] gate is > 0.
// Carousel view on the active panel always enables zoom (ignores saved preference, runtime override, and size gates).
func (a *App) effectiveZoomActivePanelLayout(width, height int, filePreviewOpen bool) bool {
	if filePreviewOpen || ui.LayoutHideInactivePanel(a.model.ViewMode, a.model.HideInactivePanel) {
		return false
	}
	if a.activePanel().CarouselMode {
		return true
	}
	orientation := a.effectivePaneSplitOrientation()
	if orientation == ui.SplitVertical {
		if a.zoomActivePanelSuppressedByTerminalHeight(height) {
			return false
		}
	} else if a.zoomActivePanelSuppressedByTerminalWidth(width) {
		return false
	}
	return a.effectiveZoomActivePanel()
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
