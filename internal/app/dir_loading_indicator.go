package app

import (
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// armDirLoadingIndicatorTimer starts (or restarts) the working-indicator delay for panelID's
// currently pending navigation load. Mirrors armIdleDiskSortTimer's arm/epoch pattern.
func (a *App) armDirLoadingIndicatorTimer(panelID int) {
	if panelID != ui.PrimaryPanel && panelID != ui.SecondaryPanel {
		return
	}
	if t := a.dirLoadIndicatorTimer[panelID]; t != nil {
		t.Stop()
	}
	a.dirLoadIndicatorEpoch[panelID]++
	epoch := a.dirLoadIndicatorEpoch[panelID]
	a.dirLoadIndicatorTimer[panelID] = time.AfterFunc(panel.LoadingIndicatorDelay, func() {
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(dirLoadingIndicatorPayload{PanelID: panelID, Epoch: epoch}))
	})
}

// applyDirLoadingIndicator is called on the main event loop once armDirLoadingIndicatorTimer's
// delay fires. A stale epoch (superseded by a newer load or already-cleared by
// invalidateDirLoadingIndicator) is a no-op.
func (a *App) applyDirLoadingIndicator(panelID int, epoch uint64) {
	if panelID != ui.PrimaryPanel && panelID != ui.SecondaryPanel {
		return
	}
	if a.dirLoadIndicatorEpoch[panelID] != epoch {
		return
	}
	p := a.panelByID(panelID)
	if p.ListingPending {
		p.ShowLoadingIcon = true
	}
}

// invalidateDirLoadingIndicator cancels panelID's pending working-indicator timer (if any) and
// bumps its epoch so a timer already in flight becomes a no-op when it fires.
func (a *App) invalidateDirLoadingIndicator(panelID int) {
	if panelID != ui.PrimaryPanel && panelID != ui.SecondaryPanel {
		return
	}
	if t := a.dirLoadIndicatorTimer[panelID]; t != nil {
		t.Stop()
		a.dirLoadIndicatorTimer[panelID] = nil
	}
	a.dirLoadIndicatorEpoch[panelID]++
}
