package app

import (
	"context"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// treeChildLoadPayload carries the result of an async tree-mode directory expansion fetch back
// to the main thread. Mirrors remotePanelLoadPayload in remote_panel.go, one level narrower in
// scope (a single subdirectory fetch, not a whole-panel navigation) — no generation counter is
// needed here; panel.ApplyTreeChildLoad's findTreeNode + Loading check is a sufficient staleness
// guard for this per-node case.
type treeChildLoadPayload struct {
	panelID int
	dirID   string
	entries []localfs.Entry
	err     error
}

func (a *App) wireTreeChildLoaders() {
	a.model.Primary.ScheduleTreeChildLoad = a.treeChildLoadScheduler(ui.PrimaryPanel)
	a.model.Secondary.ScheduleTreeChildLoad = a.treeChildLoadScheduler(ui.SecondaryPanel)
}

// treeChildLoadScheduler builds the panel.TreeChildLoadScheduler for panelID, reading the
// directory the same way the previous synchronous path (State.loadTreeChildren ->
// fetchBackendEntries) did — FetchListing + fsbackend.ToPanelEntries, so hidden-file/gitignore
// options and local/remote backends behave identically — except the listing snapshot now carries
// a real timeout (SFTP.ListTimeoutSecs, the same config value the panel-level remote loader
// already uses) instead of the synchronous path's hardcoded 0/disabled, and the fetch itself runs
// on a goroutine instead of blocking the UI thread.
func (a *App) treeChildLoadScheduler(panelID int) panel.TreeChildLoadScheduler {
	return func(req panel.TreeChildLoadRequest) bool {
		snap := a.panelByID(panelID).ListingRefreshSnapshot(req.Loc, time.Duration(a.config.SFTP.ListTimeoutSecs)*time.Second)
		go func() {
			_ = a.treeExpandAllPool.Acquire(context.Background())
			defer a.treeExpandAllPool.Release()
			backendEntries, _, _, _, err := panel.FetchListing(context.Background(), snap)
			var entries []localfs.Entry
			if err == nil {
				entries, err = fsbackend.ToPanelEntries(backendEntries)
			}
			// PostEventWait is deprecated as "unsafe" (it can block indefinitely if the main loop
			// stalls) but that block-until-delivered behavior is exactly what's needed here:
			// PostEvent silently drops the event when tcell's fixed-size queue is full, which is
			// what let treeExpandQuiet get stuck > 0 under a burst of concurrent fetches.
			a.screen.PostEventWait(tcell.NewEventInterrupt(treeChildLoadPayload{ //nolint:staticcheck // SA1019: guaranteed delivery required, see comment above
				panelID: panelID,
				dirID:   req.DirID,
				entries: entries,
				err:     err,
			}))
		}()
		return true
	}
}

func (a *App) applyTreeChildLoad(p treeChildLoadPayload) bool {
	pan := a.panelByID(p.panelID)
	if !pan.ApplyTreeChildLoad(p.dirID, p.entries, p.err, a.panelViewportRows(p.panelID)) {
		return false
	}
	if p.err != nil {
		a.setErrorMessage("Expand failed", p.err)
	}
	return true
}
