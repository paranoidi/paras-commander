package app

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/ui"
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
	ui.PaintFindDialog(a.screen, layout, &a.model.FindDialog, a.styles, a.model.ShowFileIcons, a.model.DiskUsage, a.config.DiskUsageDescendIntoMountPoints, a.diskUsageIgnore)
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

func (a *App) render() {
	a.stopDiskUsageRedrawDebounce()
	a.syncCarouselChildPreviewCoalesceFlags()
	a.model.PanelZoomActivePercent = a.config.UI.PanelZoomActivePercent
	a.model.PanelZoomInactivePercent = a.config.UI.PanelZoomInactivePercent
	a.model.ShrunkenShowsNameOnly = a.config.UI.ShrunkenShowsNameOnly
	sb, _ := uiscrollbar.ParseStyle(a.config.UI.PanelScrollbar)
	a.model.PanelScrollbar = uiscrollbar.EffectiveStyle(sb)
	a.model.PanelScrollbarInactive = a.config.UI.PanelScrollbarInactive
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
	a.model.DiskUsageDescendIntoMountPoints = a.config.DiskUsageDescendIntoMountPoints
	a.model.DiskUsageGoduIgnore = a.diskUsageIgnore
	a.commandsMu.RLock()
	a.model.FilePreviewDraw = a.model.FilePreview
	previewOpen := a.model.FilePreviewDraw.Open || a.model.QuickViewDisplayActive()
	a.model.FullscreenFilePreviewDraw = a.model.FullscreenFilePreview
	a.commandsMu.RUnlock()
	if a.model.ViewMode == ui.ViewFilePreview {
		a.clampFullscreenFilePreviewScroll()
	}
	w, _ := a.screen.Size()
	a.model.PanelZoomEnabled = a.effectiveZoomActivePanelLayout(w, previewOpen)
	a.applyCarouselPanelZoomPercents(w)
	if a.model.ViewMode == ui.ViewCommands {
		a.commandsMu.RLock()
		a.model.CommandsDisplay = append([]ui.CommandRunEntry(nil), a.model.CommandsList...)
		a.commandsMu.RUnlock()
	} else {
		a.model.CommandsDisplay = nil
	}
	if a.model.FileDialog.Open && a.model.FileDialog.DialogType == ui.FileDialogMassRename {
		a.recomputeMassRenamePreview()
	}
	ui.Render(a.screen, a.model, a.styles)
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
	if h == a.lastScreenContentHash {
		return
	}
	a.lastScreenContentHash = h
	a.screen.Show()
}

// emitScreenAfterPartialPaint runs after targeted SetContent calls (e.g. menu-bar spinner) so
// the hash cache stays aligned with what the terminal displays.
func (a *App) emitScreenAfterPartialPaint() {
	a.screen.Show()
	if a.config.UI.ScreenRenderHashCache {
		a.lastScreenContentHash = ui.HashScreenLogical(a.screen)
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
	return ui.CalculateLayout(width, height, a.model.MenuBarLayoutReserved(), a.panelWidthSplit(width, filePreviewOpen))
}

// applyCarouselPanelZoomPercents widens the active column when carousel view is on so three panes fit.
func (a *App) applyCarouselPanelZoomPercents(totalWidth int) {
	if !a.activePanel().CarouselMode || !a.model.PanelZoomEnabled {
		return
	}
	minPct := panelcarousel.MinActiveWidthPercent(totalWidth)
	if a.model.PanelZoomActivePercent < minPct {
		a.model.PanelZoomActivePercent = minPct
		a.model.PanelZoomInactivePercent = 100 - minPct
	}
}

func (a *App) panelWidthSplit(width int, filePreviewOpen bool) ui.PanelWidthSplit {
	zoom := a.effectiveZoomActivePanelLayout(width, filePreviewOpen)
	activePct := a.model.PanelZoomActivePercent
	inactivePct := a.model.PanelZoomInactivePercent
	if activePct <= 0 || inactivePct <= 0 {
		activePct = a.config.UI.PanelZoomActivePercent
		inactivePct = a.config.UI.PanelZoomInactivePercent
	}
	if zoom && a.activePanel().CarouselMode {
		minPct := panelcarousel.MinActiveWidthPercent(width)
		if activePct < minPct {
			activePct = minPct
			inactivePct = 100 - activePct
		}
	}
	return ui.PanelWidthSplit{
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
	return a.config.UI.ZoomActivePanel
}

func (a *App) zoomActivePanelSuppressedByTerminalWidth(width int) bool {
	n := a.config.UI.ZoomActivePanelDisabledAboveWidth
	return n > 0 && width >= n
}

// effectiveZoomActivePanelLayout is the zoom flag used for layout and Model.PanelZoomEnabled:
// preference from effectiveZoomActivePanel, disabled while preview/quick view owns the split,
// and disabled on wide terminals when [ui].zoom_active_panel_disabled_above_width is > 0.
// Carousel view on the active panel always enables zoom (ignores saved preference, runtime override, and width gate).
func (a *App) effectiveZoomActivePanelLayout(width int, filePreviewOpen bool) bool {
	if filePreviewOpen || ui.LayoutHideInactivePanel(a.model.ViewMode, a.model.HideInactivePanel) {
		return false
	}
	if a.activePanel().CarouselMode {
		return true
	}
	if a.zoomActivePanelSuppressedByTerminalWidth(width) {
		return false
	}
	if a.activePanel().CarouselMode {
		return true
	}
	return a.effectiveZoomActivePanel()
}

// toggleRuntimeZoomActivePanel flips the zoom split the user currently sees (saved
// [ui].zoom_active_panel plus optional runtime-only override). When the flipped state
// matches the saved config value, the override is cleared so nil always means "follow saved".
func (a *App) toggleRuntimeZoomActivePanel() {
	effective := a.effectiveZoomActivePanel()
	next := !effective
	saved := a.config.UI.ZoomActivePanel
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
