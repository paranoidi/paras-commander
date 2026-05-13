package app

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) render() {
	a.stopDiskUsageRedrawDebounce()
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

func (a *App) layoutForTerminalSize(width, height int) ui.Layout {
	return ui.CalculateLayout(width, height, a.model.MenuBarLayoutReserved())
}
