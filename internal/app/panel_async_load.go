package app

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// coalescingQueue holds async completions so a burst of goroutines posts at most one tcell
// interrupt (see treeChildResultQueue). tcell's event channel is a fixed 10-slot buffer;
// PostEvent is silently dropped when it is full.
type coalescingQueue[T any] struct {
	mu      sync.Mutex
	pending []T
	posted  bool
}

func (q *coalescingQueue[T]) push(p T) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pending = append(q.pending, p)
	if q.posted {
		return false
	}
	q.posted = true
	return true
}

func (q *coalescingQueue[T]) drain() []T {
	q.mu.Lock()
	defer q.mu.Unlock()
	items := q.pending
	q.pending = nil
	q.posted = false
	return items
}

// asyncWakeQueues is per-App storage for the listing/git/carousel coalescing queues.
type asyncWakeQueues struct {
	panel    [ui.PanelIDCount]coalescingQueue[panelAsyncLoadPayload]
	git      coalescingQueue[gitStatusPayload]
	carousel [ui.PanelIDCount]coalescingQueue[carouselSnapshotPayload]
}

func asyncWakes(a *App) *asyncWakeQueues {
	return &a.asyncWakes
}

func postGuaranteedWake(screen tcell.Screen, payload any) {
	// PostEventWait is deprecated as "unsafe" (it can block indefinitely if the main loop stalls)
	// but blocking here is safe: at most one goroutine is ever inside this call per queue, and
	// guaranteed delivery is required so the queue's posted flag cannot get stuck true with
	// nothing to drain it. See treeChildResultQueue.
	screen.PostEventWait(tcell.NewEventInterrupt(payload)) //nolint:staticcheck // SA1019: guaranteed delivery required, see comment above
}

// fetchListingForAsyncLoad is panel.FetchListing behind a package-level seam so tests can
// substitute a fake (e.g. one that blocks forever) without touching the real filesystem.
var fetchListingForAsyncLoad = panel.FetchListing

// asyncListingResult is what raceAsyncListingFetch hands to onResult: the fetched listing plus,
// when requested, the directory's filesystem probes (panel.PathProbes) computed on the same
// background goroutine — so applying the result on the UI goroutine needs no filesystem calls.
type asyncListingResult struct {
	loc                  pathloc.Path
	entries              []fsbackend.Entry
	gitignoreActive      bool
	dotfilesHiddenActive bool
	probes               *panel.PathProbes
	// treeChildren holds prefetched child listings for the request's TreePrefetch dirs that
	// finished within the time budget; the rest fall back to the async per-child path.
	treeChildren map[string]panel.TreePrefetchResult
	err          error
}

// treePrefetchBudgetDivisor splits the time left of the listing timeout: the prefetch waits at
// most 1/divisor of it, so the give-up timer can never fire while prefetch is still collecting
// and turn an otherwise good root listing into a timeout error.
const treePrefetchBudgetDivisor = 2

// treeRefreshPrio maps a refresh tier to its queue priority.
func treeRefreshPrio(t panel.TreeRefreshTier) treeListPrio {
	switch t {
	case panel.TreeRefreshCaret:
		return prioCaret
	case panel.TreeRefreshVisible:
		return prioVisible
	}
	return prioOffscreen
}

// userTreeRefreshReqs requests ids at user priority (navigation re-entry).
func userTreeRefreshReqs(ids []string) []treeListReq {
	reqs := make([]treeListReq, len(ids))
	for i, id := range ids {
		reqs[i] = treeListReq{id: id, prio: prioUser}
	}
	return reqs
}

type treeListReq struct {
	id   string
	prio treeListPrio
}

// prefetchTreeChildren lists reqs through the shared tree-list queue (FetchListing +
// ToPanelEntries, like treeChildLoader) and returns whichever finished within wait. Unfinished
// directories are omitted; queued jobs that start after the deadline do no I/O.
func (a *App) prefetchTreeChildren(snap panel.ListingRefreshSnapshot, reqs []treeListReq, wait time.Duration) map[string]panel.TreePrefetchResult {
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	snap.ClimbToExistingAncestor = false
	snap.ListTimeout = wait
	fetch := fetchListingForAsyncLoad // read once here: tests swap the package seam
	var mu sync.Mutex
	out := make(map[string]panel.TreePrefetchResult, len(reqs))
	var wg sync.WaitGroup
	for _, req := range reqs {
		id := req.id
		loc, err := pathloc.Parse(id)
		if err != nil {
			continue
		}
		wg.Add(1)
		childSnap := snap
		childSnap.Loc = loc
		a.treeListQ.push(req.prio, func() {
			defer wg.Done()
			if ctx.Err() != nil {
				return
			}
			backendEntries, _, _, _, err := fetch(ctx, childSnap)
			var entries []localfs.Entry
			if err == nil {
				entries, err = fsbackend.ToPanelEntries(backendEntries)
			}
			if ctx.Err() != nil {
				return // missed the budget; the async child path takes over
			}
			mu.Lock()
			out[id] = panel.TreePrefetchResult{Entries: entries, Err: err}
			mu.Unlock()
		})
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
	}
	mu.Lock()
	defer mu.Unlock()
	return maps.Clone(out)
}

// raceAsyncListingFetch runs snap's fetch off the UI thread, racing it against a give-up timer
// (timeout). Go cannot cancel a goroutine parked inside a real blocking syscall, so whichever of
// {fetch, timeout} finishes first "wins" and calls onResult exactly once, with err set to a
// timeout error for the timer side; the loser's outcome (a stuck fetch that does eventually
// return) is silently dropped, and the timer is stopped once the fetch wins so it doesn't sit in
// the runtime's timer heap for the rest of its duration. withProbes additionally runs
// panel.ProbeListingPath (statfs, device stat, git work-tree lookup — each a round trip on a
// network mount) on the fetch goroutine, for callers that go on to ApplyListing the result; the
// heavy volume probes are skipped while a job saturates that volume, matching what
// ApplyListing's SuppressHeavyPathProbes gate would decide.
func (a *App) raceAsyncListingFetch(snap panel.ListingRefreshSnapshot, timeout time.Duration, withProbes bool, prefetch []string, onResult func(asyncListingResult)) {
	start := time.Now()
	var settled atomic.Bool
	var timer atomic.Pointer[time.Timer] // set right after time.AfterFunc below; both post() callers only ever run after that
	post := func(res asyncListingResult) {
		if settled.CompareAndSwap(false, true) {
			if t := timer.Load(); t != nil {
				t.Stop()
			}
			onResult(res)
		}
	}
	timer.Store(time.AfterFunc(timeout, func() {
		post(asyncListingResult{err: fmt.Errorf("listing timed out after %s", timeout)})
	}))
	go func() {
		entries, loc, gitignoreActive, dotfilesHiddenActive, err := fetchListingForAsyncLoad(context.Background(), snap)
		res := asyncListingResult{loc: loc, entries: entries, gitignoreActive: gitignoreActive, dotfilesHiddenActive: dotfilesHiddenActive, err: err}
		if err == nil && withProbes {
			res.probes = a.probeListingPath(loc)
		}
		if err == nil && len(prefetch) > 0 {
			if wait := (timeout - time.Since(start)) / treePrefetchBudgetDivisor; wait > 0 {
				res.treeChildren = a.prefetchTreeChildren(snap, userTreeRefreshReqs(prefetch), wait)
			}
		}
		post(res)
	}()
}

// probeListingPath runs panel.ProbeListingPath for loc off the UI goroutine, skipping the heavy
// volume probes when a job is saturating that volume (the same rule as suppressHeavyPathProbes).
func (a *App) probeListingPath(loc pathloc.Path) *panel.PathProbes {
	includeVolume := true
	if host, err := loc.FilePath(); err == nil && a.pathVolumeContendsWithActiveJob(host) {
		includeVolume = false
	}
	pr := panel.ProbeListingPath(loc, includeVolume)
	return &pr
}

type panelAsyncLoadPayload struct {
	panelID int
	gen     uint64
	req     panel.AsyncLoadRequest
	res     asyncListingResult
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
		a.raceAsyncListingFetch(snap, timeout, true, req.TreePrefetch, func(res asyncListingResult) {
			p := panelAsyncLoadPayload{
				panelID: panelID,
				gen:     gen,
				req:     req,
				res:     res,
			}
			if asyncWakes(a).panel[panelID].push(p) {
				postGuaranteedWake(a.screen, panelAsyncLoadPayload{panelID: panelID})
			}
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
	items := asyncWakes(a).panel[p.panelID].drain()
	if len(items) == 0 {
		return a.applyOnePanelAsyncLoad(p)
	}
	changed := false
	for _, item := range items {
		if a.applyOnePanelAsyncLoad(item) {
			changed = true
		}
	}
	return changed
}

func (a *App) applyOnePanelAsyncLoad(p panelAsyncLoadPayload) bool {
	if a.panelAsyncLoadGen[p.panelID].Load() != p.gen {
		if p.req.HistoryVisit != 0 {
			if pan := a.panelByID(p.panelID); pan != nil {
				pan.RevertHistoryVisit(p.req.HistoryPath, p.req.HistoryVisit)
			}
		}
		return false
	}
	isOverlay := p.panelID == ui.QuickViewOverlayPanel || p.panelID == ui.DedupBrowsePanel
	if !a.syntheticPanelActive(p.panelID) {
		return false
	}
	pan := a.panelByID(p.panelID)
	pan.ListingPending = false
	pan.ListingPendingPath = ""
	pan.ShowLoadingIcon = false
	a.invalidateDirLoadingIndicator(p.panelID)
	if p.res.err != nil {
		if p.req.Rollback != nil {
			p.req.Rollback()
		}
		if isOverlay {
			// The overlay held the previous directory's rows while this load was in flight
			// (stale-while-revalidate); don't leave them masquerading as the failed target.
			pan.Path = p.req.Loc
			pan.Entries = nil
			pan.Cursor = 0
			pan.ScrollOffset = 0
		} else {
			a.setErrorMessage("List failed", p.res.err)
		}
		return true
	}
	// Same-directory listing that started before an in-memory listing mutation (rename/mkdir
	// insert): drop it so a stale pre-mutation snapshot doesn't overwrite the newer rows.
	// Navigation applies (loc != Path) still land even if epoch moved — Path is still the old
	// directory until ApplyListing.
	if p.req.Loc.Equal(pan.Path) && p.req.ListingEpoch != pan.ListingEpoch {
		return true
	}
	pan.GitignoreActive = p.res.gitignoreActive
	pan.DotfilesHiddenActive = p.res.dotfilesHiddenActive
	if err := pan.ApplyListingPrefetched(p.res.loc, p.res.entries, p.req.SelectedName, p.req.ViewportRows, p.req.IndexFallback, p.req.CenterRecalledCursor, p.res.probes, p.res.treeChildren); err != nil {
		if p.req.Rollback != nil {
			p.req.Rollback()
		}
		if !isOverlay {
			a.setErrorMessage("List failed", err)
		}
		return true
	}
	a.startPrefetchedTreeDiskScans(p.panelID, p.res.treeChildren)
	if p.req.SyncHistoryHead && pan.HistoryIndex == 0 && len(pan.History) > 0 {
		pan.History[0] = pan.PathString()
	}
	if p.req.OnApplied != nil {
		p.req.OnApplied()
	}
	return true
}
