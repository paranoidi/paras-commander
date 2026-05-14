package app

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) render() {
	a.stopDiskUsageRedrawDebounce()
	a.model.PanelZoomActivePercent = a.config.UI.PanelZoomActivePercent
	a.model.PanelZoomInactivePercent = a.config.UI.PanelZoomInactivePercent
	a.model.ShrunkenShowsNameOnly = a.config.UI.ShrunkenShowsNameOnly
	// Reconcile derived model (latched panel sync, disk-usage idle-sort hooks, etc.)
	// before painting so the frame matches post-mutation state. The Run loop also calls
	// reconcileAfterEvent() after each event; without this ordering, render() would run
	// first on key handling and latched sync would appear one selection behind until the
	// next redraw.
	a.reconcileAfterEvent()
	a.model.MenuBarPermission = a.menuBarPermissionText()
	a.model.MenuBarJobsAttention = a.menuBarJobsAttentionText()
	a.model.MenuBarActivitySpinner = a.menuBarSpinnerBusy()
	a.model.FooterKeys = a.activeFooterKeys()
	a.model.DiskUsageDescendIntoMountPoints = a.config.DiskUsageDescendIntoMountPoints
	a.model.DiskUsageGoduIgnore = a.diskUsageIgnore
	a.commandsMu.RLock()
	a.model.FilePreviewDraw = a.model.FilePreview
	previewOpen := a.model.FilePreviewDraw.Open || a.model.QuickViewEnabled
	a.model.FullscreenFilePreviewDraw = a.model.FullscreenFilePreview
	a.commandsMu.RUnlock()
	if a.model.ViewMode == ui.ViewFilePreview {
		a.clampFullscreenFilePreviewScroll()
	}
	a.model.PanelZoomEnabled = a.effectiveZoomActivePanel() && !previewOpen
	if a.model.ViewMode == ui.ViewCommands {
		a.commandsMu.RLock()
		a.model.CommandsDisplay = append([]ui.CommandRunEntry(nil), a.model.CommandsList...)
		a.commandsMu.RUnlock()
	} else {
		a.model.CommandsDisplay = nil
	}
	ui.Render(a.screen, a.model, a.styles)
	a.armSpinnerRedrawTimer()
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
	return ui.CalculateLayout(width, height, a.model.MenuBarLayoutReserved(), ui.PanelWidthSplit{
		Zoom:            ui.PanelZoomSplitsColumns(a.model.ViewMode, a.effectiveZoomActivePanel() && !filePreviewOpen),
		ActivePanel:     a.model.ActivePanel,
		ActivePercent:   a.config.UI.PanelZoomActivePercent,
		InactivePercent: a.config.UI.PanelZoomInactivePercent,
	})
}

func (a *App) layoutForTerminalSize(width, height int) ui.Layout {
	return a.layoutForTerminalSizePreview(width, height, a.filePreviewOpen() || a.model.QuickViewEnabled)
}

func (a *App) effectiveZoomActivePanel() bool {
	if a.zoomActivePanelOverride != nil {
		return *a.zoomActivePanelOverride
	}
	return a.config.UI.ZoomActivePanel
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
