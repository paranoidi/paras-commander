package app

import (
	"context"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type panelRefreshTickPayload struct{}

type panelRefreshApplyPayload struct {
	PanelID              int
	Path                 pathloc.Path
	Entries              []fsbackend.Entry
	GitignoreActive      bool
	DotfilesHiddenActive bool
	ListingEpoch         uint64
}

func (a *App) runPanelRefreshTicker(interval time.Duration, stop <-chan struct{}) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			_ = a.screen.PostEvent(tcell.NewEventInterrupt(panelRefreshTickPayload{}))
		}
	}
}

func (a *App) handlePanelRefreshTick() {
	if a.model.ViewMode != ui.ViewBrowser || a.model.ModalDialogOpen() {
		return
	}
	a.schedulePanelListingRefresh(ui.PrimaryPanel)
	a.schedulePanelListingRefresh(ui.SecondaryPanel)
}

func (a *App) schedulePanelListingRefresh(panelID int) {
	p := a.panelByID(panelID)
	if p == nil || p.Path.IsZero() || p.ListingPending {
		return
	}
	if panelID < 0 || panelID > ui.SecondaryPanel {
		return
	}
	if a.pathVolumeContendsWithActiveJob(p.PathString()) {
		// A job is already saturating this volume; the next tick retries.
		return
	}
	if !a.panelRefreshInFlight[panelID].CompareAndSwap(false, true) {
		return
	}
	snap := p.ListingRefreshSnapshot(p.Path, time.Duration(a.config.SFTP.ListTimeoutSecs)*time.Second)
	path := p.Path
	epoch := p.ListingEpoch
	baseline := panel.BackendEntriesFromPanel(p.Entries)
	go func(panelID int, snap panel.ListingRefreshSnapshot, path pathloc.Path, epoch uint64, baseline []fsbackend.Entry) {
		defer a.panelRefreshInFlight[panelID].Store(false)
		entries, listingLoc, gitignoreActive, dotfilesHiddenActive, err := panel.FetchListing(context.Background(), snap)
		if err != nil {
			return
		}
		if fsbackend.EntriesListingEqual(entries, baseline) {
			return
		}
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(panelRefreshApplyPayload{
			PanelID:              panelID,
			Path:                 listingLoc,
			Entries:              entries,
			GitignoreActive:      gitignoreActive,
			DotfilesHiddenActive: dotfilesHiddenActive,
			ListingEpoch:         epoch,
		}))
	}(panelID, snap, path, epoch, baseline)
}

func (a *App) applyPanelListingRefresh(p panelRefreshApplyPayload) bool {
	pan := a.panelByID(p.PanelID)
	if pan == nil {
		return false
	}
	if !pan.Path.Equal(p.Path) {
		return false
	}
	if p.ListingEpoch != pan.ListingEpoch {
		return false
	}
	if fsbackend.EntriesListingEqual(p.Entries, panel.BackendEntriesFromPanel(pan.Entries)) {
		return false
	}
	pan.GitignoreActive = p.GitignoreActive
	pan.DotfilesHiddenActive = p.DotfilesHiddenActive
	dirty, err := pan.ApplyPeriodicRefresh(p.Path, p.Entries, a.panelViewportRows(p.PanelID))
	if err != nil {
		return false
	}
	return dirty
}
