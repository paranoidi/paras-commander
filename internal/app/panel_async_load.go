package app

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// fetchListingForAsyncLoad is panel.FetchListing behind a package-level seam so tests can
// substitute a fake (e.g. one that blocks forever) without touching the real filesystem.
var fetchListingForAsyncLoad = panel.FetchListing

// raceAsyncListingFetch runs snap's fetch off the UI thread, racing it against a give-up timer
// (timeout). Go cannot cancel a goroutine parked inside a real blocking syscall, so whichever of
// {fetch, timeout} finishes first "wins" and calls onResult exactly once, with err set to a
// timeout error for the timer side; the loser's outcome (a stuck fetch that does eventually
// return) is silently dropped, and the timer is stopped once the fetch wins so it doesn't sit in
// the runtime's timer heap for the rest of its duration.
func (a *App) raceAsyncListingFetch(snap panel.ListingRefreshSnapshot, timeout time.Duration, onResult func(loc pathloc.Path, entries []fsbackend.Entry, gitignoreActive, dotfilesHiddenActive bool, err error)) {
	var settled atomic.Bool
	var timer atomic.Pointer[time.Timer] // set right after time.AfterFunc below; both post() callers only ever run after that
	post := func(loc pathloc.Path, entries []fsbackend.Entry, gitignoreActive, dotfilesHiddenActive bool, err error) {
		if settled.CompareAndSwap(false, true) {
			if t := timer.Load(); t != nil {
				t.Stop()
			}
			onResult(loc, entries, gitignoreActive, dotfilesHiddenActive, err)
		}
	}
	timer.Store(time.AfterFunc(timeout, func() {
		post(pathloc.Path{}, nil, false, false, fmt.Errorf("listing timed out after %s", timeout))
	}))
	go func() {
		entries, loc, gitignoreActive, dotfilesHiddenActive, err := fetchListingForAsyncLoad(context.Background(), snap)
		post(loc, entries, gitignoreActive, dotfilesHiddenActive, err)
	}()
}

type panelAsyncLoadPayload struct {
	panelID              int
	gen                  uint64
	req                  panel.AsyncLoadRequest
	loc                  pathloc.Path
	entries              []fsbackend.Entry
	gitignoreActive      bool
	dotfilesHiddenActive bool
	err                  error
}

func (a *App) wireAsyncPanelLoaders() {
	a.model.Primary.ScheduleAsyncLoad = a.asyncLoadScheduler(ui.PrimaryPanel)
	a.model.Secondary.ScheduleAsyncLoad = a.asyncLoadScheduler(ui.SecondaryPanel)
	a.model.Primary.OnAsyncLoadPending = func() { a.armDirLoadingIndicatorTimer(ui.PrimaryPanel) }
	a.model.Secondary.OnAsyncLoadPending = func() { a.armDirLoadingIndicatorTimer(ui.SecondaryPanel) }
}

// asyncLoadScheduler runs a directory listing off the UI thread for panelID (one of
// ui.PrimaryPanel, ui.SecondaryPanel, or ui.QuickViewOverlayPanel — see
// internal/apphandler/preview.Handler.initQuickViewDirOverlayFromFollower for the overlay's own
// wiring). A local path can stall just as badly as a remote one (network mount, autofs trigger) —
// see raceAsyncListingFetch for how the give-up timeout works. This is independent of
// panelAsyncLoadGen, which separately drops a result superseded by a newer navigation.
func (a *App) asyncLoadScheduler(panelID int) panel.AsyncLoadScheduler {
	return func(req panel.AsyncLoadRequest) bool {
		gen := a.panelAsyncLoadGen[panelID].Add(1)
		timeout := time.Duration(a.config.SFTP.ListTimeoutSecs) * time.Second
		snap := a.panelByID(panelID).ListingRefreshSnapshot(req.Loc, timeout)
		a.raceAsyncListingFetch(snap, timeout, func(loc pathloc.Path, entries []fsbackend.Entry, gitignoreActive, dotfilesHiddenActive bool, err error) {
			_ = a.screen.PostEvent(tcell.NewEventInterrupt(panelAsyncLoadPayload{
				panelID:              panelID,
				gen:                  gen,
				req:                  req,
				loc:                  loc,
				entries:              entries,
				gitignoreActive:      gitignoreActive,
				dotfilesHiddenActive: dotfilesHiddenActive,
				err:                  err,
			}))
		})
		return true
	}
}

// applyPanelAsyncLoad applies an async directory-listing result to the panel it targets — one of
// the two real panels or the QuickViewDirOverlay (panelID == ui.QuickViewOverlayPanel). The
// overlay gets two deliberate deviations from real-panel handling, both preserved from before the
// panelByID generalization: failures are dropped silently rather than surfaced via
// setErrorMessage (a background preview load failing shouldn't spam an error toast for something
// the user didn't directly request), and a result is dropped once the overlay has been
// deactivated (closed) even if its generation still matches, since the overlay — unlike the two
// real panels — can go from "the thing this load was for" to "not currently shown" without any
// new load being scheduled to bump the generation counter.
func (a *App) applyPanelAsyncLoad(p panelAsyncLoadPayload) bool {
	if a.panelAsyncLoadGen[p.panelID].Load() != p.gen {
		return false
	}
	isOverlay := p.panelID == ui.QuickViewOverlayPanel
	if isOverlay && !a.model.QuickViewDirOverlayActive {
		return false
	}
	pan := a.panelByID(p.panelID)
	pan.ListingPending = false
	pan.ListingPendingPath = ""
	pan.ShowLoadingGlyph = false
	a.invalidateDirLoadingIndicator(p.panelID)
	if p.err != nil {
		if p.req.Rollback != nil {
			p.req.Rollback()
		}
		if !isOverlay {
			a.setErrorMessage("List failed", p.err)
		}
		return true
	}
	// Same-directory listing that started before an optimistic mutation: drop it so pruned
	// rows are not resurrected. Navigation applies (loc != Path) still land even if epoch
	// moved — Path is still the old directory until ApplyListing.
	if p.req.Loc.Equal(pan.Path) && p.req.ListingEpoch != pan.ListingEpoch {
		return true
	}
	pan.GitignoreActive = p.gitignoreActive
	pan.DotfilesHiddenActive = p.dotfilesHiddenActive
	if err := pan.ApplyListing(p.loc, p.entries, p.req.SelectedName, p.req.ViewportRows, p.req.IndexFallback, p.req.CenterRecalledCursor); err != nil {
		if p.req.Rollback != nil {
			p.req.Rollback()
		}
		if !isOverlay {
			a.setErrorMessage("List failed", err)
		}
		return true
	}
	if p.req.SyncHistoryHead && pan.HistoryIndex == 0 && len(pan.History) > 0 {
		pan.History[0] = pan.PathString()
	}
	if p.req.OnApplied != nil {
		p.req.OnApplied()
	}
	return true
}
