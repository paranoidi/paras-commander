package app

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// jobsHost implements apphandler/jobs.Host for *App.
type jobsHost struct{ app *App }

func (h jobsHost) LayoutForTerminalSize(w, height int) ui.Layout {
	return h.app.layoutForTerminalSize(w, height)
}

func (h jobsHost) SetTransientMessage(text string, urgency ui.MessageUrgency) {
	h.app.setTransientMessage(text, urgency)
}

func (h jobsHost) SetUnsupportedMessage(msg string) {
	h.app.setUnsupportedMessage(msg)
}

func (h jobsHost) RefreshBothPanels() { h.app.refreshBothPanels() }

func (h jobsHost) RequestBothPanelsVolumeSpaceRefreshAsync() {
	h.app.requestBothPanelsVolumeSpaceRefreshAsync()
}

func (h jobsHost) ActivePanel() *panel.State { return h.app.activePanel() }

func (h jobsHost) ActivePanelSources() []string { return h.app.activePanelSources() }

func (h jobsHost) InactivePanel() *panel.State { return h.app.inactivePanel() }

func (h jobsHost) LeftPanel() *panel.State { return &h.app.model.Left }

func (h jobsHost) RightPanel() *panel.State { return &h.app.model.Right }

func (h jobsHost) OpenTransferDialogSelfCopyRename(kind ui.TransferKind, absDest, source string) {
	h.app.openTransferDialogSelfCopyRename(kind, absDest, source)
}

func (h jobsHost) HandleQuit() bool { return h.app.handleQuit() }

func (h jobsHost) HandleQuitImmediate() bool { return h.app.handleQuitImmediate() }

func (h jobsHost) OpenMenu() { h.app.openMenu() }

func (h jobsHost) OpenMenuByShortcut(shortcut rune) bool { return h.app.openMenuByShortcut(shortcut) }

func (h jobsHost) Dispatch(actionID string) { h.app.dispatch(actionID) }

func (h jobsHost) TryDispatchAuxiliaryScreens(actionID string) bool {
	return h.app.tryDispatchAuxiliaryScreens(actionID)
}

func (h jobsHost) ActionFromKeyEvent(ev *tcell.EventKey) string { return h.app.actionFromKeyEvent(ev) }

func (h jobsHost) SetJobFailedTransientMessage(err error, fallback string) {
	log := fmt.Sprintf("Job failed: %s", jobFailureLogDetail(err, fallback))
	banner := fmt.Sprintf("Job failed: %s", jobFailureBannerDetail(err, fallback))
	h.app.setTransientMessageBanner(log, banner, ui.MessageUrgencyError)
}

func (h jobsHost) DevMode() bool { return h.app.devMode }
