package preview

import (
	"cmp"
	"os"
	"runtime"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	previewrun "github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/preview/prefetch"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// ensurePrefetch starts, stops, or restarts the prefetch engine to match live config.
func (h *Handler) ensurePrefetch() {
	cfg := h.host.Config().Preview
	if !cfg.Prefetch || !cfg.Images {
		h.stopPrefetch()
		return
	}
	want := prefetchEngineConfig(cfg)
	if h.prefetch != nil {
		if want == h.prefetchCfg {
			return
		}
		// A settings dialog changed something the running engine froze at construction — most
		// importantly the still-decode max edge, which moves with the image protocol (the M-F3
		// image-capabilities dialog switches that at runtime, and Sixel under tmux clamps to
		// [preview].tmux_sixel_max_edge_px while every other combination uses
		// [preview].image_max_edge_px). Keeping the old value means prefetch warms one
		// LoadStill key while the live preview asks for another: every image then re-decodes on
		// first selection (visible as the prefetch loading glyph flashing on an entry the warm
		// tint already called preloaded). Restart so both sides agree on the keys again.
		h.stopPrefetch()
	}
	h.prefetchCfg = want
	h.prefetch = prefetch.NewEngine(h.ctx, want, func() {
		h.syncPrefetchLoadingMarks()
		h.postRenderWake()
	})
}

// prefetchEngineConfig derives the engine config from live preview settings. The two max-edge
// values resolve exactly as a live request's do (previewRequest → runImageCtx), so prefetched
// entries share the cache keys the foreground path will look up.
func prefetchEngineConfig(cfg config.PreviewConfig) prefetch.Config {
	protocol := previewrun.ResolveImageProtocol(cfg, os.Getenv)
	inTmux := os.Getenv("TMUX") != ""
	return prefetch.Config{
		Workers:           effectivePrefetchWorkers(cfg.PrefetchWorkers, runtime.GOMAXPROCS(0)),
		MemoryMaxMB:       cfg.PrefetchMemoryMaxMB,
		RenderCacheMaxMB:  cfg.RenderCacheMaxMB,
		VideoDiskMaxMB:    cfg.VideoThumbCacheMaxMB,
		ImageMaxEdgePx:    previewrun.EffectiveStillMaxEdge(cfg, protocol, inTmux),
		VideoMaxEdgePx:    previewrun.EffectiveVideoThumbMaxEdge(cfg, protocol, inTmux),
		VideoThumbCols:    cfg.VideoThumbCols,
		VideoThumbRows:    cfg.VideoThumbRows,
		VideoThumbWorkers: cfg.VideoThumbWorkers,
	}
}

// effectivePrefetchWorkers leaves at least one CPU free for the main/UI goroutine — the fixed
// decode/resize/encode cost per prefetched image (internal/preview/raster.go) is pure-Go and
// CPU-heavy, and saturating every core with it starves input handling badly enough to blow past
// the quick-view debounce window during rapid key-repeat scrolling. The configured value remains
// the upper bound; only the effective worker count is capped.
func effectivePrefetchWorkers(configured, gomaxprocs int) int {
	return min(configured, max(1, gomaxprocs-1))
}

func (h *Handler) stopPrefetch() {
	if h.prefetch == nil {
		return
	}
	h.prefetch.Close()
	h.prefetch = nil
	// Clear the skip-rebuild guard too: a restart (ensurePrefetch noticing changed settings)
	// leaves the caret exactly where it was, so without this the next
	// SchedulePrefetchFromActivePanel call would short-circuit and the fresh engine would sit
	// idle until the user moved the cursor.
	h.prefetchLastSurfaceActive = false
	h.prefetchLastPath = pathloc.Path{}
	h.prefetchLastCursor = 0
	h.prefetchLastEntryCount = 0
	h.mu.Lock()
	h.model.PreviewPrefetchLoading = nil
	h.model.PreviewPrefetchWarm = nil
	h.prefetchLastEntries = nil
	h.prefetchLastBox = nil
	h.mu.Unlock()
}

// syncPrefetchLoadingMarks copies in-flight paths into the model for row suffix paint. It also
// schedules a debounced warm-icon snapshot refresh (scheduleWarmMapRebuild), since this is the
// Cache's OnChange callback and can fire from a background prefetch-worker goroutine whenever a
// decode/render completes — not just when the cursor moves.
func (h *Handler) syncPrefetchLoadingMarks() {
	if h.prefetch == nil {
		h.mu.Lock()
		h.model.PreviewPrefetchLoading = nil
		h.mu.Unlock()
		h.scheduleWarmMapRebuild()
		return
	}
	paths := h.prefetch.Cache().SnapshotInFlight()
	m := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		m[p] = struct{}{}
	}
	h.mu.Lock()
	h.model.PreviewPrefetchLoading = m
	h.mu.Unlock()
	h.scheduleWarmMapRebuild()
}

// prefetchWarmMapDebounceDelay is short enough that the warm-icon tint still feels responsive,
// but long enough to coalesce a burst of near-simultaneous decode/render completions into one
// rebuild instead of one per completion.
const prefetchWarmMapDebounceDelay = 20 * time.Millisecond

// scheduleWarmMapRebuild debounces rebuildPrefetchWarmMap — see prefetchWarmMapDebounce's doc
// comment on Handler.
func (h *Handler) scheduleWarmMapRebuild() {
	h.prefetchWarmMapDebounce.Arm(prefetchWarmMapDebounceDelay, func() {
		h.rebuildPrefetchWarmMap()
		h.postRenderWake()
	})
}

// rebuildPrefetchWarmMap recomputes the debug preloaded/not-preloaded icon-tint snapshot from the
// most recently scheduled window and render box (prefetchLastEntries/prefetchLastBox). Called both
// after a full SchedulePrefetchFromActivePanel rebuild and from syncPrefetchLoadingMarks (the
// Cache's OnChange handler), so the tint also refreshes as background work completes between
// cursor moves, not only when the cursor itself moves.
func (h *Handler) rebuildPrefetchWarmMap() {
	if h.prefetch == nil {
		h.mu.Lock()
		h.model.PreviewPrefetchWarm = nil
		h.mu.Unlock()
		return
	}
	h.mu.RLock()
	entries := h.prefetchLastEntries
	box := h.prefetchLastBox
	h.mu.RUnlock()
	warm := make(map[string]struct{}, len(entries))
	for _, ent := range entries {
		if h.prefetch.IsEntryWarm(ent, box) {
			warm[ent.Path] = struct{}{}
		}
	}
	h.mu.Lock()
	h.model.PreviewPrefetchWarm = warm
	h.mu.Unlock()
}

// mediaCache returns the shared MediaCache when prefetch is enabled.
func (h *Handler) mediaCache() previewrun.MediaCache {
	h.ensurePrefetch()
	if h.prefetch == nil {
		return nil
	}
	return h.prefetch.Cache()
}

// activeRenderBox returns the exact pixel box the currently active preview surface (quick view or
// carousel — whichever is driving prefetch, see prefetchSurfaceActive) would render an image into,
// for prefetch.Engine to eagerly warm Cache.LoadRender. Returns nil when no box is determinable.
func (h *Handler) activeRenderBox(quickView bool) *prefetch.RenderBox {
	var tw, contentH int
	var ok bool
	if quickView {
		tw, contentH, ok = h.inactivePanelPreviewLayoutMetrics(true)
	} else {
		tw, contentH, ok = h.carouselChildPreviewLayoutMetrics()
	}
	if !ok || tw < 1 || contentH < 1 {
		return nil
	}
	if _, ttyOK := h.screen.Tty(); !ttyOK {
		return nil
	}
	cfg := h.host.Config().Preview
	if !cfg.Images {
		return nil
	}
	proto := previewrun.ResolveImageProtocol(cfg, os.Getenv)
	if proto == previewpanel.ImageProtocolNone {
		return nil
	}
	cw, ch := previewpanel.CellPixelDims(h.screen)
	return &prefetch.RenderBox{
		Proto:              proto,
		UnicodePlaceholder: proto == previewpanel.ImageProtocolKitty && previewrun.TmuxSupportsKittyUnicodePlaceholders(os.Getenv, cfg),
		InTmux:             os.Getenv("TMUX") != "",
		MaxPxW:             tw * cw,
		MaxPxH:             contentH * ch,
	}
}

// SchedulePrefetchFromActivePanel rebuilds the near-caret prefetch queue from the active listing.
// When prefetch is gated to preview surfaces (default), clears the queue unless quick view is
// latched or the active panel is in carousel mode — unless [preview].prefetch_always is on.
func (h *Handler) SchedulePrefetchFromActivePanel() {
	h.ensurePrefetch()
	if h.prefetch == nil {
		return
	}
	if h.prefetchNavCoalesceHeld() {
		// A quick-view or carousel nav-coalesce debounce is pending — the user is still
		// actively moving the caret and hasn't settled on an entry yet. Hold off dispatching
		// any decode work until that debounce fires (ApplyQuickViewPreviewFlush /
		// ApplyCarouselPreviewFlush clear the flag), so rapid key-repeat scrolling never starts
		// background decodes for entries the caret is only passing through. prefetchLastPath
		// stays at its pre-scroll value, so the first call after the debounce fires sees the
		// caret position as changed and rebuilds for real.
		return
	}
	surfaceActive := h.prefetchSurfaceActive()
	if !surfaceActive {
		if h.prefetchLastSurfaceActive {
			h.prefetch.Schedule(nil, 0, 0, 0, nil)
			h.prefetchLastSurfaceActive = false
			h.prefetchLastPath = pathloc.Path{}
			h.prefetchLastEntryCount = 0
			h.mu.Lock()
			h.prefetchLastEntries = nil
			h.prefetchLastBox = nil
			h.mu.Unlock()
			h.rebuildPrefetchWarmMap()
		}
		return
	}
	p := h.host.ActivePanel()
	if p == nil {
		return
	}
	// Skip the cache-lookup/sort/Schedule rebuild when nothing has moved since the previous
	// call — reconcileAfterEvent runs this on every input event, including repeats that don't
	// touch the active panel at all, so this keeps that common case free. VisibleEntryCount is
	// also compared so a listing change that doesn't move the caret or path (e.g. M-. toggling
	// hidden/gitignored files) still triggers a rebuild for the now-different set of entries.
	entryCount := p.VisibleEntryCount()
	if surfaceActive == h.prefetchLastSurfaceActive && p.Path == h.prefetchLastPath &&
		p.Cursor == h.prefetchLastCursor && entryCount == h.prefetchLastEntryCount {
		return
	}
	dir := 0
	if p.Path == h.prefetchLastPath {
		dir = cmp.Compare(p.Cursor, h.prefetchLastCursor)
	}
	h.prefetchLastSurfaceActive = true
	h.prefetchLastPath = p.Path
	h.prefetchLastCursor = p.Cursor
	h.prefetchLastEntryCount = entryCount

	window := h.host.Config().Preview.PrefetchWindow
	pageSize := max(h.host.ActiveViewportRows(), 1) // matches panel.State.Page's own floor-at-1
	lo := p.Cursor - pageSize - window
	hi := p.Cursor + pageSize + window
	start := max(lo, 0)
	end := min(hi+1, p.VisibleEntryCount())
	entries := make([]localfs.Entry, 0, end-start)
	for i := start; i < end; i++ {
		ent, _, ok := p.VisibleEntry(i)
		if !ok {
			continue
		}
		entries = append(entries, ent)
	}
	var box *prefetch.RenderBox
	switch {
	case h.model.QuickViewEnabled:
		box = h.activeRenderBox(true)
	case p.CarouselMode:
		box = h.activeRenderBox(false)
	}
	h.mu.Lock()
	h.prefetchLastEntries = entries
	h.prefetchLastBox = box
	h.mu.Unlock()
	h.rebuildPrefetchWarmMap()

	h.prefetch.ScheduleFromListing(entries, p.Cursor-start, window, dir, pageSize, box)
}

// prefetchNavCoalesceHeld reports whether a quick-view or carousel nav-coalesce debounce is
// currently pending, i.e. the caret is mid-scroll and hasn't settled yet.
func (h *Handler) prefetchNavCoalesceHeld() bool {
	return h.quickViewNavSkipReconcile.Load() || h.carouselPreviewNavSkipSnapshot.Load()
}

// prefetchSurfaceActive reports whether background prefetch should run for the current UI.
func (h *Handler) prefetchSurfaceActive() bool {
	cfg := h.host.Config().Preview
	if cfg.PrefetchAlways {
		return true
	}
	if h.model.QuickViewEnabled {
		return true
	}
	p := h.host.ActivePanel()
	return p != nil && p.CarouselMode
}
