package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// indexDriftPayload carries one panel's directory-drift result (subdirectory name to differs,
// unchecked or indexed).
type indexDriftPayload struct {
	panelID int
	dir     pathloc.Path
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

// indexChangedPayload asks the event loop to refresh what shows index data (refreshIndexViews),
// after the index changed (a scan or prune ended, a scan tagged a path, an entry was removed or
// moved).
type indexChangedPayload struct{}

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
	a.fileIndex.OnChange = func() {
		forget()
		if a.screen != nil {
			_ = a.screen.PostEvent(tcell.NewEventInterrupt(indexChangedPayload{}))
		}
	}
}

// reloadIndexPanels re-reads every panel that shows index data: browsing with fileindex, or
// showing the index itself (fileindex://), whose listing is the index and so follows the changes
// jobs make on disk through the fschange sink (UI goroutine).
func (a *App) reloadIndexPanels() {
	for _, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		if p := a.panelByID(id); (p.WithIndex || p.Path.Scheme() == pathloc.SchemeFileIndex) && !p.ListingPending {
			_ = p.Refresh(a.panelViewportRows(id))
		}
	}
}

// refreshIndexViews re-reads the panels browsing with fileindex and both panels' meta columns,
// so index state and the tags column follow an index change (UI goroutine).
func (a *App) refreshIndexViews() {
	a.reloadIndexPanels()
	a.metaCtrl.RefreshPanel(ui.PrimaryPanel)
	a.metaCtrl.RefreshPanel(ui.SecondaryPanel)
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
			host, err := fileindex.IndexPath(req.Dir.String())
			if err != nil {
				return
			}
			dirs, err := a.fileIndex.DirDrift(ctx, filepath.Clean(host), req.Subdirs, req.MaxDepth)
			if err != nil || len(dirs) == 0 {
				return
			}
			if asyncWakes(a).indexDrift.push(indexDriftPayload{panelID: panelID, dir: req.Dir, dirs: dirs, explicit: req.Explicit}) {
				postGuaranteedWake(a.screen, indexDriftPayload{})
			}
		}()
		return true
	}
}

// applyIndexDrift applies drift results on the UI goroutine; panels that left the directory and
// panels no longer browsing with fileindex are dropped.
func (a *App) applyIndexDrift(p indexDriftPayload) bool {
	items := asyncWakes(a).indexDrift.drain()
	if len(items) == 0 {
		items = []indexDriftPayload{p}
	}
	changed := false
	for _, it := range items {
		pan := a.panelByID(it.panelID)
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
		a.refreshIndexViews()
		return eventOutcome{}, true
	}
	return eventOutcome{}, false
}
