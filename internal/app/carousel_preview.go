package app

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// carouselPreviewFlushPayload reloads the carousel child side preview after file-list debounce.
type carouselPreviewFlushPayload struct {
	gen uint64
}

func (a *App) clearCarouselPreviewDebounce() {
	a.carouselPreviewDebounce.Clear()
	a.carouselPreviewDebounceGen.Add(1)
	a.carouselPreviewNavSkipSnapshot.Store(false)
}

// clearCarouselPreviewNavCoalesce stops pending carousel side-preview coalesce.
func (a *App) clearCarouselPreviewNavCoalesce() {
	a.clearCarouselPreviewDebounce()
}

func (a *App) carouselPreviewNavCoalesceContext() bool {
	if a.model.ViewMode != ui.ViewBrowser ||
		!a.activePanel().CarouselMode ||
		a.model.ActiveSubFocus != ui.SubFocusFileList ||
		a.model.Menu.Open ||
		a.model.ModalDialogOpen() {
		return false
	}
	p := a.activePanel()
	eligible := a.carouselFilePreviewEligible()
	if !panelcarousel.ShowChildPreviewColumn(*p, a.model.QuickViewDisplayActive(), eligible) {
		return false
	}
	kind := panelcarousel.ChildPreviewKindFor(*p, a.model.QuickViewDisplayActive(), eligible)
	return kind == panelcarousel.ChildPreviewDirectoryListing || kind == panelcarousel.ChildPreviewFile
}

func (a *App) scheduleCarouselPreviewDebounceTimer(gen uint64) {
	delay := time.Duration(a.config.UI.KeyRepeatDebounceMS) * time.Millisecond
	a.carouselPreviewDebounce.Reset(delay, func() {
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(carouselPreviewFlushPayload{gen: gen}))
	})
}

// beginCarouselPreviewNavCoalesce marks the next paint(s) to reuse the cached child listing.
// Call before moving the file-list cursor so the first coalesced frame after a non-coalesced period
// (e.g. Enter into a directory) does not paint an empty child column.
func (a *App) beginCarouselPreviewNavCoalesce() bool {
	if a.config.UI.KeyRepeatDebounceMS <= 0 {
		return false
	}
	if !a.carouselPreviewNavCoalesceContext() {
		return false
	}
	a.carouselPreviewNavSkipSnapshot.Store(true)
	a.syncCarouselChildPreviewCoalesceFlags()
	return true
}

// ensureCarouselChildCacheBeforeListNav builds the child preview cache when the center cursor
// is on a directory but the cache is cold (common right after chdir invalidation).
func (a *App) ensureCarouselChildCacheBeforeListNav() {
	if !a.carouselPreviewNavCoalesceContext() {
		return
	}
	p := a.activePanel()
	eligible := a.carouselFilePreviewEligible()
	if panelcarousel.ChildPreviewKindFor(*p, a.model.QuickViewDisplayActive(), eligible) != panelcarousel.ChildPreviewDirectoryListing {
		return
	}
	if p.CarouselSideCache.ChildOK {
		return
	}
	wasCoalesce := p.CarouselChildPreviewCoalesce
	p.CarouselChildPreviewCoalesce = false
	_, _ = p.SnapshotChild(a.activeViewportRows())
	p.CarouselChildPreviewCoalesce = wasCoalesce
}

func (a *App) armCarouselPreviewNavCoalesceAfterListNav() {
	if !a.beginCarouselPreviewNavCoalesce() {
		return
	}
	gen := a.carouselPreviewDebounceGen.Add(1)
	a.scheduleCarouselPreviewDebounceTimer(gen)
}

func (a *App) applyCarouselPreviewFlush(p carouselPreviewFlushPayload) bool {
	if p.gen != a.carouselPreviewDebounceGen.Load() {
		return false
	}
	a.carouselPreviewNavSkipSnapshot.Store(false)
	a.loadCarouselChildPreviewFromDisk()
	return true
}

// loadCarouselChildPreviewFromDisk reloads the carousel child preview after nav coalesce ends.
func (a *App) loadCarouselChildPreviewFromDisk() {
	if !a.carouselPreviewNavCoalesceContext() {
		return
	}
	a.syncCarouselChildPreviewCoalesceFlags()
	p := a.activePanel()
	eligible := a.carouselFilePreviewEligible()
	kind := panelcarousel.ChildPreviewKindFor(*p, a.model.QuickViewDisplayActive(), eligible)
	switch kind {
	case panelcarousel.ChildPreviewDirectoryListing:
		_, _ = p.SnapshotChild(a.activeViewportRows())
	case panelcarousel.ChildPreviewFile:
		a.applyCarouselFilePreviewAfterFlush()
	}
}

// carouselPreviewHeldListNav reports file-list nav keys while carousel child preview coalesce may apply.
func (a *App) carouselPreviewHeldListNav(resolvedAction string, event *tcell.EventKey) bool {
	if a.config.UI.KeyRepeatDebounceMS <= 0 || !a.activePanel().CarouselMode {
		return false
	}
	return a.panelSyncFollowHeldListNav(resolvedAction, event)
}

// syncCarouselChildPreviewCoalesceFlags sets child-preview coalesce before painting carousel columns.
func (a *App) syncCarouselChildPreviewCoalesceFlags() {
	coalesce := a.carouselPreviewNavSkipSnapshot.Load() && a.carouselPreviewNavCoalesceContext()
	a.model.Primary.CarouselChildPreviewCoalesce = coalesce && a.model.Primary.CarouselMode && a.model.ActivePanel == ui.PrimaryPanel
	a.model.Secondary.CarouselChildPreviewCoalesce = coalesce && a.model.Secondary.CarouselMode && a.model.ActivePanel == ui.SecondaryPanel
}
