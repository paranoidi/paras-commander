package app

import (
	"context"
	"maps"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
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
	// TreeChildren is every loaded dir's children (baseline overlaid with fresh results);
	// TreeFetched is only the freshly listed subset (the ones worth a disk scan).
	TreeChildren map[string]panel.TreePrefetchResult
	TreeFetched  map[string]panel.TreePrefetchResult
	// TreeChanged: the root listing is unchanged but a prefetched nested directory differs.
	TreeChanged bool
}

// treeChildrenChanged reports whether any fetched child listing differs from its baseline.
// Ids absent from children (missed the budget) count as unchanged.
func treeChildrenChanged(children map[string]panel.TreePrefetchResult, baseline map[string][]localfs.Entry) bool {
	for id, res := range children {
		old, ok := baseline[id]
		if res.Err != nil {
			if ok {
				return true
			}
			continue
		}
		if !ok || !fsbackend.EntriesListingEqual(panel.BackendEntriesFromPanel(res.Entries), panel.BackendEntriesFromPanel(old)) {
			return true
		}
	}
	return false
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
	timeout := time.Duration(a.config.SFTP.ListTimeoutSecs) * time.Second
	snap := p.ListingRefreshSnapshot(p.Path, timeout)
	plan := p.TreeRefreshPlan(a.panelViewportRows(panelID), config.DefaultTreeRefreshOffscreenDirsPerTick)
	path := p.Path
	epoch := p.ListingEpoch
	baseline := panel.BackendEntriesFromPanel(p.Entries)
	childBaseline := p.LoadedTreeChildren()
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
		rootChanged := !fsbackend.EntriesListingEqual(entries, baseline)
		if !rootChanged && len(plan) == 0 {
			return
		}
		// The apply re-roots the tree, so fetch the planned expansions now (same budget rule as
		// raceAsyncListingFetch) and the refreshed tree lands in one paint.
		var fetched map[string]panel.TreePrefetchResult
		if len(plan) > 0 {
			if wait := (timeout - time.Since(start)) / treePrefetchBudgetDivisor; wait > 0 {
				reqs := make([]treeListReq, len(plan))
				for i, r := range plan {
					reqs[i] = treeListReq{id: r.ID, prio: treeRefreshPrio(r.Tier)}
				}
				fetched = a.prefetchTreeChildren(snap, reqs, wait)
			}
		}
		// ponytail: off-screen expanded dirs are refreshed round-robin, one per tick, so off-screen
		// changes can take a while to show. Upgrade path: fs watching (inotify).
		treeChanged := !rootChanged && treeChildrenChanged(fetched, childBaseline)
		if !rootChanged && !treeChanged {
			return
		}
		// The re-root reattaches every expanded dir from memory: baseline for dirs not re-listed
		// this tick, fresh results on top. Baseline-filled dirs diff equal, so they get no marks.
		children := make(map[string]panel.TreePrefetchResult, len(childBaseline)+len(fetched))
		for id, es := range childBaseline {
			children[id] = panel.TreePrefetchResult{Entries: es}
		}
		maps.Copy(children, fetched)
		probes := a.probeListingPath(listingLoc)
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(panelRefreshApplyPayload{
			PanelID:              panelID,
			Path:                 listingLoc,
			Entries:              entries,
			GitignoreActive:      gitignoreActive,
			DotfilesHiddenActive: dotfilesHiddenActive,
			ListingEpoch:         epoch,
			Probes:               probes,
			TreeChildren:         children,
			TreeFetched:          fetched,
			TreeChanged:          treeChanged,
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
	if !p.TreeChanged && fsbackend.EntriesListingEqual(p.Entries, panel.BackendEntriesFromPanel(pan.Entries)) {
		return false
	}
	pan.GitignoreActive = p.GitignoreActive
	pan.DotfilesHiddenActive = p.DotfilesHiddenActive
	dirty, err := pan.ApplyPeriodicRefresh(p.Path, p.Entries, a.panelViewportRows(p.PanelID), p.Probes, p.TreeChildren, p.TreeChanged)
	if err != nil {
		return false
	}
	a.startPrefetchedTreeDiskScans(p.PanelID, p.TreeFetched)
	return dirty
}
