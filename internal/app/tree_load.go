package app

import (
	"context"
	"sync"
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

// treeChildResultsReadyPayload is the tcell interrupt payload posted to wake the main loop into
// draining treeChildResults. It carries no data itself — results live in the queue, not the event —
// so a burst of fetch completions coalesces into a single pending post (see treeChildResultQueue).
type treeChildResultsReadyPayload struct{}

// treeChildResultQueue coalesces async tree-child-load completions from potentially thousands of
// concurrent fetch goroutines (a very wide directory level under ExpandAllTreeFully) into at most
// one pending tcell event at a time. tcell's own event channel is a fixed 10-slot buffer shared
// with real terminal input (see tScreen.evch); key presses are posted to it with the non-blocking
// PostEvent and are silently dropped when the channel is full (tScreen.PostEvent's ErrEventQFull
// path). Posting one interrupt per directory fetch kept that channel saturated for the whole
// cascade, dropping/delaying the user's actual keystrokes — this queue means only one goroutine
// ever has a post in flight; every other completion just appends and returns.
type treeChildResultQueue struct {
	mu      sync.Mutex
	pending []treeChildLoadPayload
	posted  bool
}

// push appends p and reports whether the caller must post the wake event (true only for whichever
// goroutine transitions the queue from empty/drained to having a pending post).
func (q *treeChildResultQueue) push(p treeChildLoadPayload) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pending = append(q.pending, p)
	if q.posted {
		return false
	}
	q.posted = true
	return true
}

// drain removes and returns every pending result, resetting posted so the next push triggers a
// fresh wake event.
func (q *treeChildResultQueue) drain() []treeChildLoadPayload {
	q.mu.Lock()
	defer q.mu.Unlock()
	items := q.pending
	q.pending = nil
	q.posted = false
	return items
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
			backendEntries, _, _, _, err := panel.FetchListing(context.Background(), snap)
			var entries []localfs.Entry
			if err == nil {
				entries, err = fsbackend.ToPanelEntries(backendEntries)
			}
			// Only the goroutine that actually flips the queue from drained to pending posts the
			// wake event — every other concurrent completion just appends and returns, so a wide
			// directory level never posts more than one pending interrupt at a time (see
			// treeChildResultQueue). PostEventWait is deprecated as "unsafe" (it can block
			// indefinitely if the main loop stalls) but blocking here is safe: at most one goroutine
			// is ever inside this call, and guaranteed delivery is required so the queue's posted
			// flag can't get stuck true with nothing to drain it.
			if a.treeChildResults.push(treeChildLoadPayload{
				panelID: panelID,
				dirID:   req.DirID,
				entries: entries,
				err:     err,
			}) {
				a.screen.PostEventWait(tcell.NewEventInterrupt(treeChildResultsReadyPayload{})) //nolint:staticcheck // SA1019: guaranteed delivery required, see comment above
			}
		}()
		return true
	}
}

// applyTreeChildResults drains every tree-child-load result that piled up in treeChildResults
// since the last wake event (see treeChildResultQueue) and applies them all in one pass, so a
// wide cascade level's thousands of individual fetch completions collapse into a single render
// instead of one interrupt-handler round trip per directory.
//
// Each item is applied via panel.ApplyTreeChildLoad without forcing a rebuild of its own: a
// per-item progress repaint (as an earlier version of this code did) reintroduces the same
// problem the batching above was meant to fix — a large drained batch would run its own
// full-tree treeflat.Flatten/rebuildFilter every ~200ms of elapsed *processing* time, which
// happily elapses many times over within a single tight drain loop on a big tree, hogging the
// main goroutine for seconds without ever calling PollEvent and starving real terminal input
// worse than before. Instead, the whole batch gets at most one rebuild: if none of the drained
// items already triggered one (pan.ApplyTreeChildLoad returns true once a level fully lands or
// for a non-coalesced single expand), PeekTreeRows runs once at the end so the loading icons for
// whatever just got dispatched are still visible. Batches naturally self-pace: a new wake event
// can't be posted until this drain finishes (treeChildResultQueue.posted resets in drain()), so a
// slow tree gets frequent small batches and a fast one gets fewer, larger ones — either way, one
// rebuild per drain, not one per fetch.
func (a *App) applyTreeChildResults() bool {
	items := a.treeChildResults.drain()
	changed := false
	touchedPanel := -1
	for _, p := range items {
		touchedPanel = p.panelID
		pan := a.panelByID(p.panelID)
		if pan.ApplyTreeChildLoad(p.dirID, p.entries, p.err, a.panelViewportRows(p.panelID)) {
			changed = true
			if p.err != nil {
				a.setErrorMessage("Expand failed", p.err)
			}
		}
	}
	if !changed && touchedPanel >= 0 {
		pan := a.panelByID(touchedPanel)
		pan.PeekTreeRows(a.panelViewportRows(touchedPanel))
		changed = true
	}
	return changed
}
