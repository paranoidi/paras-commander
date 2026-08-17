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
}

// asyncLoadScheduler runs a directory listing off the UI thread for panelID. A local path can
// stall just as badly as a remote one (network mount, autofs trigger), and Go cannot cancel a
// goroutine parked inside a real blocking syscall — so alongside the fetch itself, a timer races
// it to give up after config.SFTP.ListTimeoutSecs and surface a timed-out navigation instead of
// leaving the panel on ListingPending forever. Whichever of {fetch, timeout} finishes first wins
// via settled; the loser's result (if the stuck fetch eventually does return) is silently
// dropped. This is independent of panelAsyncLoadGen, which still separately drops a result
// superseded by a newer navigation.
func (a *App) asyncLoadScheduler(panelID int) panel.AsyncLoadScheduler {
	return func(req panel.AsyncLoadRequest) bool {
		gen := a.panelAsyncLoadGen[panelID].Add(1)
		timeout := time.Duration(a.config.SFTP.ListTimeoutSecs) * time.Second
		snap := a.panelByID(panelID).ListingRefreshSnapshot(req.Loc, timeout)

		var settled atomic.Bool
		post := func(p panelAsyncLoadPayload) {
			if settled.CompareAndSwap(false, true) {
				_ = a.screen.PostEvent(tcell.NewEventInterrupt(p))
			}
		}

		go func() {
			entries, listingLoc, gitignoreActive, dotfilesHiddenActive, err := fetchListingForAsyncLoad(context.Background(), snap)
			post(panelAsyncLoadPayload{
				panelID:              panelID,
				gen:                  gen,
				req:                  req,
				loc:                  listingLoc,
				entries:              entries,
				gitignoreActive:      gitignoreActive,
				dotfilesHiddenActive: dotfilesHiddenActive,
				err:                  err,
			})
		}()
		time.AfterFunc(timeout, func() {
			post(panelAsyncLoadPayload{
				panelID: panelID,
				gen:     gen,
				req:     req,
				err:     fmt.Errorf("listing timed out after %s", timeout),
			})
		})
		return true
	}
}

func (a *App) applyPanelAsyncLoad(p panelAsyncLoadPayload) bool {
	if a.panelAsyncLoadGen[p.panelID].Load() != p.gen {
		return false
	}
	pan := a.panelByID(p.panelID)
	pan.ListingPending = false
	if p.err != nil {
		if p.req.Rollback != nil {
			p.req.Rollback()
		}
		a.setErrorMessage("List failed", p.err)
		return true
	}
	pan.GitignoreActive = p.gitignoreActive
	pan.DotfilesHiddenActive = p.dotfilesHiddenActive
	if err := pan.ApplyListing(p.loc, p.entries, p.req.SelectedName, p.req.ViewportRows, p.req.IndexFallback, p.req.CenterRecalledCursor); err != nil {
		if p.req.Rollback != nil {
			p.req.Rollback()
		}
		a.setErrorMessage("List failed", err)
		return true
	}
	if p.req.SyncHistoryHead && pan.HistoryIndex == 0 && len(pan.History) > 0 {
		pan.History[0] = pan.PathString()
	}
	return true
}

// quickViewAsyncLoadPayload is the async directory-listing result for the QuickViewDirOverlay.
// Tracked separately from panelAsyncLoadPayload (own gen counter, own payload type) because the
// overlay is not one of the two real panels panelByID resolves — sharing a real panel's
// scheduler/gen slot for the overlay's own Load() calls would misattribute the overlay's listing
// onto that real panel (or spuriously supersede that panel's own in-flight navigation). Mirrors
// quickViewGitStatusScheduler/quickViewGitLoadGen's existing split for the same reason.
type quickViewAsyncLoadPayload struct {
	gen                  uint64
	req                  panel.AsyncLoadRequest
	loc                  pathloc.Path
	entries              []fsbackend.Entry
	gitignoreActive      bool
	dotfilesHiddenActive bool
	err                  error
}

func (a *App) quickViewAsyncLoadScheduler() panel.AsyncLoadScheduler {
	return func(req panel.AsyncLoadRequest) bool {
		gen := a.quickViewAsyncLoadGen.Add(1)
		timeout := time.Duration(a.config.SFTP.ListTimeoutSecs) * time.Second
		snap := a.model.QuickViewDirOverlay.ListingRefreshSnapshot(req.Loc, timeout)

		var settled atomic.Bool
		post := func(p quickViewAsyncLoadPayload) {
			if settled.CompareAndSwap(false, true) {
				_ = a.screen.PostEvent(tcell.NewEventInterrupt(p))
			}
		}

		go func() {
			entries, listingLoc, gitignoreActive, dotfilesHiddenActive, err := fetchListingForAsyncLoad(context.Background(), snap)
			post(quickViewAsyncLoadPayload{
				gen:                  gen,
				req:                  req,
				loc:                  listingLoc,
				entries:              entries,
				gitignoreActive:      gitignoreActive,
				dotfilesHiddenActive: dotfilesHiddenActive,
				err:                  err,
			})
		}()
		time.AfterFunc(timeout, func() {
			post(quickViewAsyncLoadPayload{
				gen: gen,
				req: req,
				err: fmt.Errorf("listing timed out after %s", timeout),
			})
		})
		return true
	}
}

// applyQuickViewAsyncLoad merges an async directory-listing result into the QuickViewDirOverlay,
// dropping it if the overlay has since been deactivated or repopulated (a newer Load() bumped
// the shared generation counter).
func (a *App) applyQuickViewAsyncLoad(p quickViewAsyncLoadPayload) bool {
	if !a.model.QuickViewDirOverlayActive || a.quickViewAsyncLoadGen.Load() != p.gen {
		return false
	}
	ov := &a.model.QuickViewDirOverlay
	ov.ListingPending = false
	if p.err != nil {
		return true
	}
	ov.GitignoreActive = p.gitignoreActive
	ov.DotfilesHiddenActive = p.dotfilesHiddenActive
	if err := ov.ApplyListing(p.loc, p.entries, p.req.SelectedName, p.req.ViewportRows, p.req.IndexFallback, p.req.CenterRecalledCursor); err != nil {
		return true
	}
	if p.req.SyncHistoryHead && ov.HistoryIndex == 0 && len(ov.History) > 0 {
		ov.History[0] = ov.PathString()
	}
	return true
}
