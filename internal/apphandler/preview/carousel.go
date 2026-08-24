package preview

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (h *Handler) clearCarouselPreviewDebounce() {
	h.carouselPreviewDebounce.Stop()
	h.carouselPreviewDebounceGen.Add(1)
	h.carouselPreviewNavSkipSnapshot.Store(false)
}

// ClearCarouselPreviewNavCoalesce stops pending carousel side-preview coalesce.
func (h *Handler) ClearCarouselPreviewNavCoalesce() {
	h.clearCarouselPreviewDebounce()
}

func (h *Handler) carouselPreviewNavCoalesceContext() bool {
	if h.model.ViewMode != ui.ViewBrowser ||
		!h.host.ActivePanel().CarouselMode ||
		h.model.ActiveSubFocus != ui.SubFocusFileList ||
		h.model.Menu.Open ||
		h.model.ModalDialogOpen() {
		return false
	}
	p := h.host.ActivePanel()
	eligible := h.carouselFilePreviewEligible()
	if !panelcarousel.ShowChildPreviewColumn(*p, h.model.QuickViewDisplayActive(), eligible) {
		return false
	}
	kind := panelcarousel.ChildPreviewKindFor(*p, h.model.QuickViewDisplayActive(), eligible)
	return kind == panelcarousel.ChildPreviewDirectoryListing || kind == panelcarousel.ChildPreviewFile
}

func (h *Handler) scheduleCarouselPreviewDebounceTimer(gen uint64) {
	delay := time.Duration(h.host.Config().UI.KeyRepeatDebounceMS) * time.Millisecond
	h.carouselPreviewDebounce.Arm(delay, func() {
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(CarouselPreviewFlushPayload{gen: gen}))
	})
}

// BeginCarouselPreviewNavCoalesce marks the next paint(s) to reuse the cached child listing.
// Call before moving the file-list cursor so the first coalesced frame after a non-coalesced period
// (e.g. Enter into a directory) does not paint an empty child column.
func (h *Handler) BeginCarouselPreviewNavCoalesce() bool {
	if h.host.Config().UI.KeyRepeatDebounceMS <= 0 {
		return false
	}
	if !h.carouselPreviewNavCoalesceContext() {
		return false
	}
	h.carouselPreviewNavSkipSnapshot.Store(true)
	h.SyncCarouselChildPreviewCoalesceFlags()
	return true
}

// EnsureCarouselChildCacheBeforeListNav builds the child preview cache when the center cursor
// is on a directory but the cache is cold (common right after chdir invalidation).
func (h *Handler) EnsureCarouselChildCacheBeforeListNav() {
	if !h.carouselPreviewNavCoalesceContext() {
		return
	}
	p := h.host.ActivePanel()
	eligible := h.carouselFilePreviewEligible()
	if panelcarousel.ChildPreviewKindFor(*p, h.model.QuickViewDisplayActive(), eligible) != panelcarousel.ChildPreviewDirectoryListing {
		return
	}
	if p.CarouselSideCache.ChildOK {
		return
	}
	wasCoalesce := p.CarouselChildPreviewCoalesce
	p.CarouselChildPreviewCoalesce = false
	_, _ = p.SnapshotChild(h.host.ActiveViewportRows())
	p.CarouselChildPreviewCoalesce = wasCoalesce
}

// ArmCarouselPreviewNavCoalesceAfterListNav arms the carousel side-preview coalesce debounce
// after a file-list cursor move, when currently eligible.
func (h *Handler) ArmCarouselPreviewNavCoalesceAfterListNav() {
	if !h.BeginCarouselPreviewNavCoalesce() {
		return
	}
	gen := h.carouselPreviewDebounceGen.Add(1)
	h.scheduleCarouselPreviewDebounceTimer(gen)
}

// ApplyCarouselPreviewFlush applies the debounced carousel side-preview reload. Returns true when
// a repaint is needed.
func (h *Handler) ApplyCarouselPreviewFlush(p CarouselPreviewFlushPayload) bool {
	if p.gen != h.carouselPreviewDebounceGen.Load() {
		return false
	}
	h.carouselPreviewNavSkipSnapshot.Store(false)
	h.loadCarouselChildPreviewFromDisk()
	return true
}

// FlushCarouselPreviewNow applies the currently pending carousel side-preview debounce
// immediately (skips waiting for the timer), for callers that need synchronous flush semantics.
func (h *Handler) FlushCarouselPreviewNow() bool {
	return h.ApplyCarouselPreviewFlush(CarouselPreviewFlushPayload{gen: h.carouselPreviewDebounceGen.Load()})
}

// CarouselPreviewNavSkipSnapshot reports whether carousel child-preview nav coalesce is currently
// holding a pending snapshot reload (render.go and tests use this to observe coalesce state).
func (h *Handler) CarouselPreviewNavSkipSnapshot() bool {
	return h.carouselPreviewNavSkipSnapshot.Load()
}

// loadCarouselChildPreviewFromDisk reloads the carousel child preview after nav coalesce ends.
func (h *Handler) loadCarouselChildPreviewFromDisk() {
	if !h.carouselPreviewNavCoalesceContext() {
		return
	}
	h.SyncCarouselChildPreviewCoalesceFlags()
	p := h.host.ActivePanel()
	eligible := h.carouselFilePreviewEligible()
	kind := panelcarousel.ChildPreviewKindFor(*p, h.model.QuickViewDisplayActive(), eligible)
	switch kind {
	case panelcarousel.ChildPreviewDirectoryListing:
		_, _ = p.SnapshotChild(h.host.ActiveViewportRows())
	case panelcarousel.ChildPreviewFile:
		h.applyCarouselFilePreviewAfterFlush()
	}
}

// CarouselPreviewHeldListNav reports file-list nav keys while carousel child preview coalesce may apply.
func (h *Handler) CarouselPreviewHeldListNav(resolvedAction string, event *tcell.EventKey) bool {
	if h.host.Config().UI.KeyRepeatDebounceMS <= 0 || !h.host.ActivePanel().CarouselMode {
		return false
	}
	return h.host.PanelSyncFollowHeldListNav(resolvedAction, event)
}

// SyncCarouselChildPreviewCoalesceFlags sets child-preview coalesce before painting carousel columns.
func (h *Handler) SyncCarouselChildPreviewCoalesceFlags() {
	coalesce := h.carouselPreviewNavSkipSnapshot.Load() && h.carouselPreviewNavCoalesceContext()
	h.model.Primary.CarouselChildPreviewCoalesce = coalesce && h.model.Primary.CarouselMode && h.model.ActivePanel == ui.PrimaryPanel
	h.model.Secondary.CarouselChildPreviewCoalesce = coalesce && h.model.Secondary.CarouselMode && h.model.ActivePanel == ui.SecondaryPanel
}
