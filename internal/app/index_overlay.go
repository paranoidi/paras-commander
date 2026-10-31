package app

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// indexDriftPayload carries one panel's directory-drift result (names of indexed subdirectories
// that differ from the index below them).
type indexDriftPayload struct {
	panelID int
	gen     uint64
	dir     pathloc.Path
	dirs    map[string]bool
}

// indexChangedPayload asks the event loop to reload every panel browsing with fileindex, after
// the index changed (a scan or prune ended, an entry was removed or moved).
type indexChangedPayload struct{}

// indexDriftState is the per-panel generation and cancel for the drift pass: a newer
// listing (or navigation) supersedes and cancels an older one still walking.
type indexDriftState struct {
	gen    [ui.PanelIDCount]uint64
	mu     sync.Mutex
	cancel [ui.PanelIDCount]context.CancelFunc
}

// wireIndexOverlay installs the overlay and drift hooks on both panels.
func (a *App) wireIndexOverlay() {
	for _, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		p := a.panelByID(id)
		p.IndexOverlay = a.fileIndex.Overlay
		p.ScheduleIndexDrift = a.indexDriftScheduler(id)
	}
	forget := a.disk.engine.ForgetSized
	a.fileIndex.OnChange = func() {
		forget()
		if a.screen != nil {
			_ = a.screen.PostEvent(tcell.NewEventInterrupt(indexChangedPayload{}))
		}
	}
}

// reloadIndexPanels re-reads every panel that is browsing with fileindex (UI goroutine).
func (a *App) reloadIndexPanels() {
	for _, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		if p := a.panelByID(id); p.WithIndex && !p.ListingPending {
			_ = p.Refresh(a.panelViewportRows(id))
		}
	}
}

// indexDriftScheduler runs fileindex.Index.DirDrift on its own goroutine for panelID, guarded by
// a per-panel generation like gitStatusScheduler's cwd-level fetch.
func (a *App) indexDriftScheduler(panelID int) panel.IndexDriftScheduler {
	return func(req panel.IndexDriftRequest) bool {
		st := &a.indexDrift
		ctx, cancel := context.WithCancel(context.Background())
		st.mu.Lock()
		if st.cancel[panelID] != nil {
			st.cancel[panelID]()
		}
		st.cancel[panelID] = cancel
		st.gen[panelID]++
		gen := st.gen[panelID]
		st.mu.Unlock()
		go func() {
			defer cancel()
			host, err := fileindex.IndexPath(req.Dir.String())
			if err != nil {
				return
			}
			dirs, err := a.fileIndex.DirDrift(ctx, filepath.Clean(host), req.Subdirs)
			if err != nil || len(dirs) == 0 {
				return
			}
			if asyncWakes(a).indexDrift.push(indexDriftPayload{panelID: panelID, gen: gen, dir: req.Dir, dirs: dirs}) {
				postGuaranteedWake(a.screen, indexDriftPayload{})
			}
		}()
		return true
	}
}

// applyIndexDrift applies drift results on the UI goroutine; stale generations, panels that left
// the directory and panels no longer browsing with fileindex are dropped.
func (a *App) applyIndexDrift(p indexDriftPayload) bool {
	items := asyncWakes(a).indexDrift.drain()
	if len(items) == 0 {
		items = []indexDriftPayload{p}
	}
	changed := false
	for _, it := range items {
		a.indexDrift.mu.Lock()
		current := a.indexDrift.gen[it.panelID] == it.gen
		a.indexDrift.mu.Unlock()
		pan := a.panelByID(it.panelID)
		if !current || it.dirs == nil || !pan.WithIndex || !pan.Path.Equal(it.dir) {
			continue
		}
		pan.ApplyIndexDrift(it.dirs)
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
		a.reloadIndexPanels()
		return eventOutcome{}, true
	}
	return eventOutcome{}, false
}
