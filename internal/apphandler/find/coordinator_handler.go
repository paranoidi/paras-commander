package find

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fswalk"
	"github.com/paranoidi/paras-commander/internal/scan"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func New(d Deps) *Handler {
	h := &Handler{
		host:           d.Host,
		screen:         d.Screen,
		model:          d.Model,
		config:         d.Config,
		keys:           d.Keys,
		keysFindDialog: d.KeysFindDialog,
	}
	h.scan = scan.NewCoordinator(h.findWalkParams(), h.onScanEvent)
	return h
}

// WakePayload wakes PollEvent when scan coordinator events arrive.
type WakePayload struct {
	Event scan.Event
}

func (h *Handler) onScanEvent(ev scan.Event) {
	h.eventMu.Lock()
	h.pendingEvents = append(h.pendingEvents, ev)
	h.eventMu.Unlock()
	if h.wakePending.CompareAndSwap(false, true) {
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(WakePayload{}))
	}
}

func (h *Handler) drainScanEvents() []scan.Event {
	h.eventMu.Lock()
	defer h.eventMu.Unlock()
	if len(h.pendingEvents) == 0 {
		return nil
	}
	out := h.pendingEvents
	h.pendingEvents = nil
	return out
}

func (h *Handler) findStartOpts(st *dialog.FindDialogState) scan.StartOpts {
	return scan.StartOpts{
		Gen:                  h.scanGen,
		DisplayRoot:          filepath.Clean(st.RootPath),
		Roots:                h.findScopeRoots(st),
		IncludeHidden:        st.IncludeHidden,
		Gitignore:            h.host.GitignoreCache(),
		VolumeGate:           h.findVolumeGate(st),
		SelectionRoots:       st.SelectionDirRoots,
		SearchOnlySelections: st.SearchOnlySelections,
		Walk:                 h.findWalkParams(),
	}
}

func (h *Handler) startFindIndexer() {
	st := &h.model.FindDialog
	if !st.Open || st.RootPath == "" {
		return
	}
	h.scanGen++
	h.scan.Start(h.findStartOpts(st))
	st.Indexing = true
	st.IndexDone = false
	st.IndexErr = ""
}

func (h *Handler) restartFindIndexer() {
	h.stopFindIndexer()
	h.cancelPendingRank()
	st := &h.model.FindDialog
	if !st.Open || st.RootPath == "" {
		return
	}
	st.IndexErr = ""
	st.Entries = nil
	st.PathIsDir = nil
	st.PathSize = nil
	st.IndexedCount = 0
	st.Ranked = nil
	st.RankDisplayLines = nil
	st.MatchRanges = nil
	st.Selected = 0
	st.ListScroll = 0
	st.RankPending = false
	h.startFindIndexer()
}

func (h *Handler) stopFindIndexer() {
	h.scanGen++
	h.scan.Cancel()
}

func (h *Handler) syncEntriesFromScan(st *dialog.FindDialogState) {
	snap := h.scan.Snapshot()
	if len(snap) == 0 {
		st.Entries = nil
		st.PathIsDir = nil
		st.PathSize = nil
		return
	}
	entries := make([]dialog.FindEntry, len(snap))
	for i, e := range snap {
		entries[i] = dialog.FindEntry{
			RelLine: e.RelLine,
			IsDir:   e.IsDir,
			Type:    e.Type,
			Size:    e.Size,
		}
	}
	st.Entries = entries
	isDir, sizes := pathIndexFromSnapshot(st.RootPath, snap)
	st.PathIsDir = isDir
	st.PathSize = sizes
}

func (h *Handler) findWalkParams() fswalk.Params {
	return fswalk.Params{
		InitialWorkers:  h.config.FSWalk.InitialWorkers,
		MaxWorkers:      h.config.FSWalk.MaxWorkers,
		AdaptIntervalMS: h.config.FSWalk.AdaptIntervalMS,
	}
}

func pathIndexFromSnapshot(root string, snap []scan.Entry) (map[string]bool, map[string]int64) {
	isDir := make(map[string]bool, len(snap))
	sizes := make(map[string]int64)
	for _, e := range snap {
		abs := filepath.Clean(e.Path)
		if abs == "" {
			abs = filepath.Clean(filepath.Join(root, filepath.FromSlash(e.RelLine)))
		}
		isDir[abs] = e.IsDir
		if !e.IsDir && e.Size > 0 {
			sizes[abs] = e.Size
		}
	}
	if len(sizes) == 0 {
		sizes = nil
	}
	return isDir, sizes
}

func (h *Handler) finishFindIndexing(st *dialog.FindDialogState) {
	st.Indexing = false
	st.IndexDone = true
	st.WalkWorkers = 0
	h.syncEntriesFromScan(st)
	st.RankDisplayLines = nil
	if search.Parse(st.Query).Empty() {
		h.applyEmptyQueryDisplayRank(st)
	} else {
		h.scheduleFindRank(0)
	}
}

func (h *Handler) applyScanEvent(st *dialog.FindDialogState, ev scan.Event) (needRender, needSync bool) {
	if ev.Gen != h.scanGen {
		return false, false
	}
	if ev.CountUpdate {
		st.IndexedCount = ev.Count
		st.WalkWorkers = ev.WalkWorkers
		if ev.IndexActive {
			st.Indexing = true
			st.IndexDone = false
		} else if !st.IndexDone {
			st.Indexing = true
		} else {
			st.Indexing = false
		}
		if h.maybeRenderFindIndexing(st) {
			needRender = true
		}
	}
	if ev.IndexReplaced {
		h.syncEntriesFromScan(st)
		st.IndexedCount = len(st.Entries)
		st.Indexing = false
		st.IndexDone = true
		st.RankDisplayLines = nil
		h.clearFindNavIdle()
		if search.Parse(st.Query).Empty() {
			h.applyEmptyQueryDisplayRank(st)
		} else {
			h.scheduleFindRank(0)
		}
		needRender = true
		needSync = true
	}
	if ev.IndexFinished {
		if ev.IndexErr != "" {
			st.IndexErr = ev.IndexErr
			needRender = true
		}
		if st.Indexing || !st.IndexDone {
			h.finishFindIndexing(st)
			needRender = true
			needSync = true
		}
	}
	if ev.MatchResult {
		h.rankMu.Lock()
		if ev.Match.Gen == h.rankGen {
			h.pendingRank = &rankResult{
				gen:              ev.Match.Gen,
				ranked:           append([]int(nil), ev.Match.Ranked...),
				fullRanked:       append([]int(nil), ev.Match.FullRanked...),
				matchRanges:      ev.Match.MatchRanges,
				rankDisplayLines: append([]string(nil), ev.Match.DisplayRelLines...),
				entriesLen:       ev.Match.EntriesLen,
				onlyDirs:         ev.Match.OnlyDirs,
				onlyFiles:        ev.Match.OnlyFiles,
			}
		}
		h.rankMu.Unlock()
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(RankWakePayload{}))
	}
	return needRender, needSync
}

func (h *Handler) PollUpdates(_ WakePayload) bool {
	h.wakePending.Store(false)
	needRender := false
	st := &h.model.FindDialog
	for _, ev := range h.drainScanEvents() {
		r, sync := h.applyScanEvent(st, ev)
		if r {
			needRender = true
		}
		if sync && !findIndexingSkipsRank(st) {
			throttle := 0
			if st.Open && st.Indexing {
				throttle = config.DefaultFindIndexingRankThrottleMS
			}
			h.scheduleFindRank(throttle)
		}
	}
	return needRender
}
