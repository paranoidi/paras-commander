package jump

import (
	"path/filepath"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/findcrit"
	"github.com/paranoidi/paras-commander/internal/scan"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// startScan starts a fresh directories-only walk of the dialog root (JumpDisk), dropping the
// previous one and its results.
func (h *Handler) startScan() {
	st := &h.model.JumpDialog
	h.stopScan()
	h.scanGen++
	p := h.host.PanelByID(st.PanelID)
	h.scan.Start(scan.StartOpts{
		Gen:           h.scanGen,
		DisplayRoot:   st.Root,
		Roots:         []string{st.Root},
		IncludeHidden: st.IncludeHidden,
		Gitignore:     h.host.GitignoreCache(),
		VolumeGate: diskusage.ListingVolumeGate{
			Enabled: st.StayOnVolume && p.ListingDeviceValid, RefDev: p.ListingDevice, Valid: p.ListingDeviceValid,
		},
		Walk:     h.config.AdaptiveIO.Params(),
		DirsOnly: true,
		MaxDepth: st.MaxDepth,
	})
	st.Scanning, st.Scanned, st.Err = true, 0, ""
	st.Depth, st.LevelDone, st.LevelTotal = 0, 0, 0
	st.Results, st.Matched, st.Selected, st.ListScroll = nil, 0, 0, 0
	h.rankGen++
}

func (h *Handler) stopScan() {
	h.scanGen++
	h.scan.Cancel()
}

func (h *Handler) onScanEvent(ev scan.Event) {
	h.eventMu.Lock()
	h.events = append(h.events, ev)
	h.eventMu.Unlock()
	if h.wakePending.CompareAndSwap(false, true) {
		h.post(WakePayload{})
	}
}

func (h *Handler) drainEvents() []scan.Event {
	h.eventMu.Lock()
	defer h.eventMu.Unlock()
	out := h.events
	h.events = nil
	return out
}

// PollScan applies the queued scan events; it reports whether the dialog needs a repaint.
// Walk progress repaints at most every [find indexing count throttle]; results always do.
func (h *Handler) PollScan() bool {
	h.wakePending.Store(false)
	st := &h.model.JumpDialog
	repaint, rerank := false, false
	for _, ev := range h.drainEvents() {
		if ev.Gen != h.scanGen || !st.Open {
			continue
		}
		if ev.CountUpdate {
			st.Scanned = ev.Count
			st.Depth, st.LevelDone, st.LevelTotal = ev.Depth, ev.LevelDone, ev.LevelTotal
			st.Scanning = ev.IndexActive
			if time.Since(h.lastProgress) >= time.Duration(config.DefaultFindIndexingCountThrottleMS)*time.Millisecond {
				h.lastProgress = time.Now()
				repaint = true
			}
		}
		if len(ev.BatchAdded) > 0 || ev.IndexReplaced || ev.IndexFinished {
			rerank = true
		}
		if ev.IndexFinished {
			st.Depth, st.LevelDone, st.LevelTotal = 0, 0, 0
			st.Err = ev.IndexErr
			repaint = true
		}
		if ev.MatchResult && ev.Match.Gen == h.rankGen {
			h.applyMatch(ev.Match)
			repaint = true
		}
	}
	if rerank {
		h.scanProgressed(!st.Scanning)
	}
	return repaint
}

// scanProgressed re-ranks after the walked directories changed: at once when the walk finished,
// else at most every [find indexing rank throttle] (a pending debounce or throttle sees the new
// directories when it fires).
func (h *Handler) scanProgressed(finished bool) {
	if finished {
		h.stopRankTimer()
		h.requestMatch()
		return
	}
	if h.rankTimer != nil {
		return
	}
	interval := time.Duration(config.DefaultFindIndexingRankThrottleMS) * time.Millisecond
	if wait := interval - time.Since(h.lastRankSent); wait > 0 {
		h.armRankTimer(wait)
		return
	}
	h.requestMatch()
}

// requestMatch ranks the walked directories against the query on the scan coordinator; an
// empty query lists them in walk order (shallowest first).
func (h *Handler) requestMatch() {
	st := &h.model.JumpDialog
	st.Searching = true
	h.lastRankSent = time.Now()
	h.scan.RequestMatch(scan.MatchRequest{
		Gen:             h.rankGen,
		Query:           st.Query,
		Mode:            findcrit.ModeFuzzy,
		OnlyDirs:        true,
		CaseInsensitive: h.config.Filter.CaseInsensitive,
		MaxResults:      h.config.UI.Find.MaxResults,
	})
}

func (h *Handler) applyMatch(m scan.MatchOutput) {
	st := &h.model.JumpDialog
	prev, _ := st.Cursor()
	results := make([]dialog.JumpResult, len(m.Ranked))
	for i, idx := range m.Ranked {
		rel := ""
		if i < len(m.DisplayRelLines) {
			rel = m.DisplayRelLines[i]
		}
		results[i] = dialog.JumpResult{
			Path:  filepath.Join(st.Root, filepath.FromSlash(rel)),
			Rel:   rel,
			Match: m.MatchRanges[idx],
		}
	}
	st.Results, st.Matched = results, len(m.FullRanked)
	st.Searching = false
	h.keepCursor(prev.Path)
}
