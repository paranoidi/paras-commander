package preview

import (
	"os"

	"github.com/paranoidi/paras-commander/internal/localfs"
	previewrun "github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/preview/prefetch"
)

// ensurePrefetch starts or stops the prefetch engine to match live config.
func (h *Handler) ensurePrefetch() {
	cfg := h.host.Config().Preview
	if !cfg.Prefetch || !cfg.Images {
		h.stopPrefetch()
		return
	}
	if h.prefetch != nil {
		return
	}
	// Resolve the same effective still-image max-edge a live request would, so prefetched
	// cache entries share the same cache key and actually get hit.
	protocol := previewrun.ResolveImageProtocol(cfg.ImageProtocol, os.Getenv)
	inTmux := os.Getenv("TMUX") != ""
	h.prefetch = prefetch.NewEngine(h.ctx, prefetch.Config{
		Workers:        cfg.PrefetchWorkers,
		MemoryMaxMB:    cfg.PrefetchMemoryMaxMB,
		VideoDiskMaxMB: cfg.VideoThumbCacheMaxMB,
		ImageMaxEdgePx: previewrun.EffectiveStillMaxEdge(cfg, protocol, inTmux),
		VideoMaxEdgePx: previewrun.EffectiveVideoThumbMaxEdge(cfg, protocol, inTmux),
		VideoThumbCols: cfg.VideoThumbCols,
		VideoThumbRows: cfg.VideoThumbRows,
		OnChange: func() {
			h.syncPrefetchLoadingMarks()
			h.postRenderWake()
		},
	})
}

func (h *Handler) stopPrefetch() {
	if h.prefetch == nil {
		return
	}
	h.prefetch.Close()
	h.prefetch = nil
	h.mu.Lock()
	h.model.PreviewPrefetchLoading = nil
	h.mu.Unlock()
}

// syncPrefetchLoadingMarks copies in-flight paths into the model for row suffix paint.
func (h *Handler) syncPrefetchLoadingMarks() {
	if h.prefetch == nil {
		h.mu.Lock()
		h.model.PreviewPrefetchLoading = nil
		h.mu.Unlock()
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
}

// mediaCache returns the shared MediaCache when prefetch is enabled.
func (h *Handler) mediaCache() previewrun.MediaCache {
	h.ensurePrefetch()
	if h.prefetch == nil {
		return nil
	}
	return h.prefetch.Cache()
}

// SchedulePrefetchFromActivePanel rebuilds the near-caret prefetch queue from the active listing.
// When prefetch is gated to preview surfaces (default), clears the queue unless quick view is
// latched or the active panel is in carousel mode — unless [preview].prefetch_always is on.
func (h *Handler) SchedulePrefetchFromActivePanel() {
	h.ensurePrefetch()
	if h.prefetch == nil {
		return
	}
	if !h.prefetchSurfaceActive() {
		h.prefetch.Schedule(nil)
		return
	}
	p := h.host.ActivePanel()
	if p == nil {
		return
	}
	entries := make([]localfs.Entry, 0, p.VisibleEntryCount())
	for i := 0; i < p.VisibleEntryCount(); i++ {
		ent, _, ok := p.VisibleEntry(i)
		if !ok {
			continue
		}
		entries = append(entries, ent)
	}
	h.prefetch.ScheduleFromListing(entries, p.Cursor)
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
