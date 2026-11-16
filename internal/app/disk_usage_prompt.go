package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// requestDiskUsageScanForPanel is the one entry point of panel.disk-usage-scan (key and menu).
// While a disk usage scan runs or its data is on screen it asks Rescan / Clear / Cancel instead
// of starting another scan.
func (a *App) requestDiskUsageScanForPanel(panelID int) {
	scanning := a.model.DiskUsageShown && a.diskUsageScanBusy()
	if !scanning && !a.diskUsageDataShown() {
		a.startDiskUsageScanForPanel(panelID)
		return
	}
	a.model.DiskUsagePrompt = dialog.DiskUsagePromptState{
		Open: true, Focus: dialog.DiskUsageRescan, PanelID: panelID, Scanning: scanning,
	}
}

// diskUsageDataShown reports whether either panel paints the Disk Usage view (bars) now.
func (a *App) diskUsageDataShown() bool {
	if !a.model.DiskUsageShown {
		return false
	}
	return a.listingInDiskUsageScanScope(a.model.Primary.PathString()) ||
		a.listingInDiskUsageScanScope(a.model.Secondary.PathString())
}

func (a *App) handleDiskUsagePromptKey(event *tcell.EventKey) bool {
	st := &a.model.DiskUsagePrompt
	if ev, ok := dlg.HandleKey(dialog.DiskUsagePromptView(*st), &st.Focus, event); ok {
		a.applyDiskUsagePromptEvent(ev)
	}
	return false
}

// applyDiskUsagePromptEvent answers the disk usage prompt (key or click).
func (a *App) applyDiskUsagePromptEvent(ev dlg.Event) {
	if ev.ID != "" {
		a.answerDiskUsagePrompt(ev.ID)
	}
}

// answerDiskUsagePrompt closes the prompt and acts on the answer. Every way of answering (buttons,
// mnemonics, the central OK / Cancel chords) comes through here.
func (a *App) answerDiskUsagePrompt(id dlg.ID) {
	panelID := a.model.DiskUsagePrompt.PanelID
	a.model.DiskUsagePrompt = dialog.DiskUsagePromptState{}
	switch id {
	case dialog.DiskUsageRescan:
		if a.diskUsageScanBusy() {
			a.disk.engine.Abort()
			a.stopDiskUsageRedrawDebounce()
		}
		a.startDiskUsageScanForPanel(panelID)
	case dialog.DiskUsageClear:
		a.clearAllDiskUsageData()
	}
}
