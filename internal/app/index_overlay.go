package app

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// indexDriftPayload carries the end of one panel's drift pass: seq (panel.IndexDriftRequest.Seq)
// and the result (subdirectory name to differs, unchecked or indexed), nil when the pass failed
// or was cancelled.
type indexDriftPayload struct {
	panelID int
	dir     pathloc.Path
	seq     uint64
	dirs    map[string]string
	// explicit marks the result of an explicit (unlimited) request; it is never overwritten by
	// a later automatic result.
	explicit bool
}

// explicitDriftNames records, for the panel's current path and listing epoch, which directory
// names had an explicit drift requested (asked, by the selection scan) and delivered (done).
type explicitDriftNames struct {
	key         string
	asked, done map[string]bool
}

// explicitDriftFor returns panelID's record, emptied when the path or listing epoch changed.
func (a *App) explicitDriftFor(panelID int) *explicitDriftNames {
	p := a.panelByID(panelID)
	key := fmt.Sprintf("%s\x00%d", p.Path.String(), p.ListingEpoch)
	ex := &a.explicitDrift[panelID]
	if ex.key != key || ex.asked == nil {
		*ex = explicitDriftNames{key: key, asked: map[string]bool{}, done: map[string]bool{}}
	}
	return ex
}

// indexChangedPayload asks the event loop to refresh what shows index data (refreshIndexViewsIn),
// after the index changed (a scan or prune ended, a scan tagged a path, an entry was removed or
// moved). dirs names the directories changed; nil means anything may have.
type indexChangedPayload struct{ dirs []string }

// indexScanWakePayload is posted (throttled) after a running scan committed results; the directories
// wait in asyncWakeQueues.indexScan.
type indexScanWakePayload struct{}

// indexDriftState is the per-panel cancel and directory of the drift passes: walks for
// different subdirectories of one panel directory run side by side under one context, and a
// request for another directory cancels them.
type indexDriftState struct {
	mu     sync.Mutex
	cancel [ui.PanelIDCount]context.CancelFunc
	ctx    [ui.PanelIDCount]context.Context
	dir    [ui.PanelIDCount]pathloc.Path
}

// wireIndexOverlay installs the overlay and drift hooks on both panels.
func (a *App) wireIndexOverlay() {
	for _, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		p := a.panelByID(id)
		p.IndexOverlay = a.fileIndex.Overlay
		p.ScheduleIndexDrift = a.indexDriftScheduler(id)
		p.IndexAutoDriftDepth = a.config.FileIndex.AutoDriftDepth()
		p.WithIndex = a.config.FileIndex.StartBrowseWith
	}
	forget := a.disk.engine.ForgetSized
	a.fileIndex.OnChange = func(dirs []string) {
		forget()
		if a.screen != nil {
			_ = a.screen.PostEvent(tcell.NewEventInterrupt(indexChangedPayload{dirs: dirs}))
		}
	}
	a.fileIndex.OnScanCommit = a.queueIndexScanWake
}

// queueIndexScanWake records directories a running scan committed results for and, for the first of
// a window, schedules the wake: it only adds to a queue, so the scan never waits on the UI. Safe
// from any goroutine.
func (a *App) queueIndexScanWake(dirs []string) {
	first := false
	for _, d := range dirs {
		if asyncWakes(a).indexScan.push(d) {
			first = true
		}
	}
	if !first {
		return
	}
	time.AfterFunc(config.DefaultIndexScanRefreshInterval, func() {
		select {
		case <-a.jobStopCh:
			return
		default:
		}
		postGuaranteedWake(a.screen, indexScanWakePayload{})
	})
}

// handleIndexScanWake re-lists the panels showing a directory a running scan committed results for,
// so rows change state as results land. Outside the browser or behind a modal the directories are
// dropped: the scan's end refreshes everything (UI goroutine).
func (a *App) handleIndexScanWake() {
	dirs := asyncWakes(a).indexScan.drain()
	if len(dirs) == 0 || a.model.ViewMode != ui.ViewBrowser || a.model.ModalDialogOpen() {
		return
	}
	for _, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		p := a.panelByID(id)
		if !p.ShowsIndex() {
			continue
		}
		ids, matched := matchChangedDirs(p.Path, watchTreeIDs(p), dirs)
		if shown := a.fileIndex.ShownDir(p.Path); shown != "" && slices.Contains(dirs, shown) && !slices.Contains(matched, shown) {
			matched = append(matched, shown)
		}
		if len(matched) == 0 {
			continue
		}
		if a.scheduleListingRefresh(id, listingRefreshOpts{treeIDs: ids, indexProgress: true}) {
			a.queueIndexScanWake(matched) // transient skip: ask again after one more window
		}
	}
}

// indexChangeAffects reports whether a change to dirs shows in the panel: at or above its directory,
// or at one of its expanded tree directories. nil dirs means anything may have changed.
func (a *App) indexChangeAffects(p *panel.State, dirs []string) bool {
	if dirs == nil {
		return true
	}
	shown := a.fileIndex.ShownDir(p.Path)
	for _, d := range dirs {
		if shown != "" && (shown == d || strings.HasPrefix(shown, strings.TrimSuffix(d, "/")+"/")) {
			return true
		}
	}
	ids, _ := matchChangedDirs(pathloc.Path{}, watchTreeIDs(p), dirs)
	return len(ids) > 0
}

// refreshIndexViews re-reads what shows index data after a change anywhere in the index.
func (a *App) refreshIndexViews() { a.refreshIndexViewsIn(nil) }

// refreshIndexViewsIn re-reads the panels showing the index whose directory dirs affect (browsing
// with fileindex, or showing the index itself, fileindex://, whose listing follows the changes jobs
// make on disk through the fschange sink), and those panels' fileindex meta columns, so index state
// and those columns follow an index change. A listing in flight may have read the index before the
// change, so Refresh replaces it (it leaves a navigation alone). UI goroutine.
func (a *App) refreshIndexViewsIn(dirs []string) {
	for _, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		p := a.panelByID(id)
		if !a.indexChangeAffects(p, dirs) {
			continue
		}
		if p.ShowsIndex() {
			_ = p.Refresh(a.panelViewportRows(id))
		}
		a.metaCtrl.RefreshProviders(id)
	}
}

// indexDriftScheduler runs fileindex.Index.DirDrift on its own goroutine for panelID. Requests
// for the panel's current directory share one context; a request for another directory cancels
// the old one.
func (a *App) indexDriftScheduler(panelID int) panel.IndexDriftScheduler {
	return func(req panel.IndexDriftRequest) bool {
		st := &a.indexDrift
		st.mu.Lock()
		if st.cancel[panelID] == nil || !st.dir[panelID].Equal(req.Dir) {
			if st.cancel[panelID] != nil {
				st.cancel[panelID]()
			}
			st.ctx[panelID], st.cancel[panelID] = context.WithCancel(context.Background())
			st.dir[panelID] = req.Dir
		}
		ctx := st.ctx[panelID]
		st.mu.Unlock()
		go func() {
			var dirs map[string]string
			if host, err := fileindex.IndexPath(req.Dir.String()); err == nil {
				if res, err := a.fileIndex.DirDrift(ctx, filepath.Clean(host), req.Subdirs, req.MaxDepth); err == nil {
					dirs = res
				}
			}
			if asyncWakes(a).indexDrift.push(indexDriftPayload{panelID: panelID, dir: req.Dir, seq: req.Seq, dirs: dirs, explicit: req.Explicit}) {
				postGuaranteedWake(a.screen, indexDriftPayload{})
			}
		}()
		return true
	}
}

// applyIndexDrift ends the panels' wait on the finished passes and applies their results on the
// UI goroutine; results for panels that left the directory or no longer browse with fileindex are
// dropped.
func (a *App) applyIndexDrift(p indexDriftPayload) bool {
	items := asyncWakes(a).indexDrift.drain()
	if len(items) == 0 {
		items = []indexDriftPayload{p}
	}
	changed := false
	for _, it := range items {
		pan := a.panelByID(it.panelID)
		if pan.FinishIndexDrift(it.seq) {
			changed = true
		}
		if it.dirs == nil || !pan.WithIndex || !pan.Path.Equal(it.dir) {
			continue
		}
		ex := a.explicitDriftFor(it.panelID)
		res := it.dirs
		if it.explicit {
			for n := range res {
				ex.done[n] = true
			}
		} else {
			res = make(map[string]string, len(it.dirs))
			for n, st := range it.dirs {
				if !ex.done[n] {
					res[n] = st
				}
			}
		}
		pan.ApplyIndexDrift(res)
		changed = true
	}
	return changed
}

// handleIndexOverlayPayload handles the browse-with-fileindex wake payloads; ok is false for any
// other payload.
func (a *App) handleIndexOverlayPayload(data any) (eventOutcome, bool) {
	switch d := data.(type) {
	case indexDriftPayload:
		if a.applyIndexDrift(d) {
			a.render()
			return eventOutcome{didRender: true}, true
		}
		return eventOutcome{}, true
	case indexChangedPayload:
		a.refreshIndexViewsIn(d.dirs)
		return eventOutcome{}, true
	case indexScanWakePayload:
		a.handleIndexScanWake()
		return eventOutcome{}, true
	}
	return eventOutcome{}, false
}
