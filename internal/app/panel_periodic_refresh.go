package app

import (
	"context"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// fetchListingForPanelRefresh is panel.FetchListing behind a package-level seam so tests can
// substitute a fake (e.g. one that blocks) without touching the real filesystem.
var fetchListingForPanelRefresh = panel.FetchListing

type panelRefreshTickPayload struct{}

type panelRefreshApplyPayload struct {
	PanelID              int
	Path                 pathloc.Path
	Entries              []fsbackend.Entry
	GitignoreActive      bool
	DotfilesHiddenActive bool
	ListingEpoch         uint64
	Probes               *panel.PathProbes
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
	// A prior slow refresh for this panel set a start-to-start deadline; skipped ticks must
	// never flip the in-flight flag, since nothing will clear it for them.
	if time.Now().UnixNano() < a.panelRefreshNotBefore[panelID].Load() {
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
		start := time.Now()
		defer func() {
			// Record the earliest next start (start-to-start, not gap-to-gap) before clearing
			// in-flight, so a tick that fires the instant this goroutine finishes still sees the
			// deadline. A newly navigated directory deliberately inherits this deadline rather
			// than resetting it: the directory was just explicitly listed by the navigation
			// itself, and resetting here would race this deferred write.
			elapsed := time.Since(start)
			a.panelRefreshNotBefore[panelID].Store(start.Add(time.Duration(config.DefaultPanelRefreshSlowBackoffFactor) * elapsed).UnixNano())
			a.panelRefreshInFlight[panelID].Store(false)
		}()
		entries, listingLoc, gitignoreActive, dotfilesHiddenActive, err := fetchListingForPanelRefresh(context.Background(), snap)
		if err != nil {
			return
		}
		if fsbackend.EntriesListingEqual(entries, baseline) {
			return
		}
		probes := a.probeListingPath(listingLoc)
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(panelRefreshApplyPayload{
			PanelID:              panelID,
			Path:                 listingLoc,
			Entries:              entries,
			GitignoreActive:      gitignoreActive,
			DotfilesHiddenActive: dotfilesHiddenActive,
			ListingEpoch:         epoch,
			Probes:               probes,
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
	dirty, err := pan.ApplyPeriodicRefresh(p.Path, p.Entries, a.panelViewportRows(p.PanelID), p.Probes)
	if err != nil {
		return false
	}
	return dirty
}
