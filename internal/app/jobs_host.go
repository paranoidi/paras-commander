package app

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// jobsHost implements apphandler/jobs.Host for *App.
type jobsHost struct {
	appShellHost
}

func (h jobsHost) SetUnsupportedMessage(msg string) {
	h.app.setUnsupportedMessage(msg)
}

func (h jobsHost) RefreshBothPanels() { h.app.refreshBothPanels() }

func (h jobsHost) RequestBothPanelsVolumeSpaceRefreshAsync() {
	h.app.requestBothPanelsVolumeSpaceRefreshAsync()
}

func (h jobsHost) ActivePanelSources() []string { return h.app.activePanelSources() }

func (h jobsHost) InactivePanel() *panel.State { return h.app.inactivePanel() }

func (h jobsHost) PrimaryPanel() *panel.State { return &h.app.model.Primary }

func (h jobsHost) SecondaryPanel() *panel.State { return &h.app.model.Secondary }

func (h jobsHost) OpenTransferDialogSelfCopyRename(kind ui.TransferKind, absDest, source string) {
	h.app.openTransferDialogSelfCopyRename(kind, absDest, source)
}

func (h jobsHost) SetJobFailedTransientMessage(err error, fallback string) {
	log := fmt.Sprintf("Job failed: %s", jobFailureLogDetail(err, fallback))
	banner := fmt.Sprintf("Job failed: %s", jobFailureBannerDetail(err, fallback))
	h.app.setTransientMessageBanner(log, banner, ui.MessageUrgencyError)
}

func (h jobsHost) DevMode() bool { return h.app.devMode }
