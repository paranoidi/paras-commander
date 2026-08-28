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
	// Empty for directory targets, so the child-listing coalesce keeps the key-repeat delay.
	path, _ := h.carouselFilePreviewWantPath()
	delay := h.previewDebounceDelay(path)
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

// EnsureCarouselChildCacheBeforeListNav dispatches an async child preview fetch when the center
// cursor is on a directory but the cache is cold (common right after chdir invalidation).
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
	target, ok := p.CarouselChildPreviewTarget()
	if !ok {
		return
	}
	h.dispatchCarouselChildSnapshot(h.model.ActivePanel, target, h.host.ActiveViewportRows())
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
		if target, ok := p.CarouselChildPreviewTarget(); ok {
			h.dispatchCarouselChildSnapshot(h.model.ActivePanel, target, h.host.ActiveViewportRows())
		}
	case panelcarousel.ChildPreviewFile:
		h.applyCarouselFilePreviewAfterFlush()
	}
}

// carouselRetryCooldown is how long a carousel side-column fetch target is left alone after its
// fetch failed. A failure clears the in-flight marker so the target can be retried at all, but the
// dispatch trigger is a cache-validity check that runs every Run-loop pass — so without a cooldown
// a fast-failing target (permission denied, a deleted directory) would start a fetch per event.
// Retries stay frequent enough that a transient failure — the contended-volume list timeout this
// async path exists for — recovers on its own within a few seconds of the volume freeing up.
const carouselRetryCooldown = 5 * time.Second

// carouselRetryGate throttles re-dispatch of one carousel side column after a failed fetch. It is
// keyed by target so a failure never delays a fetch for a *different* directory: navigating away
// from a directory whose listing failed dispatches immediately, rather than serving out the cooldown
// earned by the previous target.
type carouselRetryGate struct {
	target string
	after  time.Time
}

// blocks reports whether target is still cooling down from an earlier failure.
func (g carouselRetryGate) blocks(target string, now time.Time) bool {
	return g.target == target && now.Before(g.after)
}

// fail starts target's cooldown.
func (g *carouselRetryGate) fail(target string, now time.Time) {
	g.target = target
	g.after = now.Add(carouselRetryCooldown)
}

// carouselSideDispatch is one panel's dispatch bookkeeping for one carousel side column: pending
// is the target an async fetch is currently in flight for, so ReconcileCarouselSidePreview — which
// runs on every Run-loop event — dispatches once per target instead of re-dispatching on every
// pass while the first fetch is still outstanding.
type carouselSideDispatch struct {
	pending string
	retry   carouselRetryGate
}

// shouldFetch reports whether target still needs an async fetch: the cache doesn't already cover
// it, no fetch for it is in flight, and it isn't cooling down from a recent failure.
func (d *carouselSideDispatch) shouldFetch(target string, cacheValid bool, now time.Time) bool {
	return !cacheValid && d.pending != target && !d.retry.blocks(target, now)
}

// carouselSideState pairs a panel's two side columns so both share one gating/retry implementation.
type carouselSideState struct {
	parent carouselSideDispatch
	child  carouselSideDispatch
}

func (h *Handler) carouselSideSlot(panelID int, isChild bool) *carouselSideDispatch {
	if isChild {
		return &h.carouselSide[panelID].child
	}
	return &h.carouselSide[panelID].parent
}

// NoteCarouselSnapshotFailed records that an async side-column fetch came back with an error
// (including the give-up timeout). Clearing the in-flight marker is what makes the failure
// recoverable: ReconcileCarouselSidePreview only dispatches when the cache is invalid AND no fetch
// is already pending for that target, and a failed fetch satisfies neither condition again on its
// own — leaving the parent column painting the last successfully fetched directory's listing
// indefinitely, since SnapshotParent reads the cache without checking which directory it holds.
// The retry gate keeps the re-dispatch from becoming a per-event fetch loop.
func (h *Handler) NoteCarouselSnapshotFailed(panelID int, isChild bool, target string) {
	if panelID < ui.PrimaryPanel || panelID > ui.SecondaryPanel {
		return
	}
	slot := h.carouselSideSlot(panelID, isChild)
	slot.pending = ""
	slot.retry.fail(target, time.Now())
}

// dispatchCarouselParentSnapshot and dispatchCarouselChildSnapshot are the single entry points for
// starting an async side-column fetch: each records the in-flight target so
// ReconcileCarouselSidePreview won't re-dispatch the same one, then hands off to the app-side
// scheduler.
func (h *Handler) dispatchCarouselParentSnapshot(panelID int, target string, viewportRows int) {
	h.carouselSideSlot(panelID, false).pending = target
	h.host.ScheduleCarouselParentSnapshot(panelID, viewportRows)
}

func (h *Handler) dispatchCarouselChildSnapshot(panelID int, target string, viewportRows int) {
	h.carouselSideSlot(panelID, true).pending = target
	h.host.ScheduleCarouselChildSnapshot(panelID, viewportRows)
}

// ReconcileCarouselSidePreview keeps panelID's carousel side-column caches in sync with the
// panel's current directory (parent column) and cursor target (child column), dispatching an
// async fetch for whichever is stale. Invoked from App.reconcileAfterEvent for both panels every
// Run-loop iteration.
//
// This is the primary dispatch path for BOTH columns, deliberately driven by cache validity
// rather than by specific input events: the cursor's target directory changes for many reasons
// besides a nav keypress — a chdir landing on a default highlight, a cursor restored from history,
// a filter or sort reordering rows, a deletion shifting the cursor — and hooking only the nav keys
// left the child column showing the previous directory's listing until the user happened to move
// the cursor off and back.
//
// The nav-coalesce debounce still owns the child column during rapid nav-key repeats; this defers
// to it (the flush dispatches instead) so held-arrow scrolling doesn't fire a fetch per step, and a
// target whose fetch just failed is held off for carouselRetryCooldown (see carouselRetryGate).
func (h *Handler) ReconcileCarouselSidePreview(panelID int) {
	parent, child := h.carouselSideSlot(panelID, false), h.carouselSideSlot(panelID, true)
	p := h.host.PanelByID(panelID)
	if p == nil || !p.CarouselMode {
		parent.pending, child.pending = "", ""
		return
	}
	now := time.Now()
	// PanelViewportRows recomputes the layout, so it is only asked for on the passes that actually
	// dispatch — not on the steady-state passes where both caches are already valid.
	if dir, ok := p.CarouselParentPreviewTarget(); !ok {
		parent.pending = ""
	} else if parent.shouldFetch(dir, p.CarouselParentCacheValid(), now) {
		h.dispatchCarouselParentSnapshot(panelID, dir, h.host.PanelViewportRows(panelID))
	}
	target, ok := p.CarouselChildPreviewTarget()
	if !ok {
		child.pending = ""
		return
	}
	// During a nav-key coalesce burst the debounce flush dispatches the child fetch instead.
	if h.carouselPreviewNavSkipSnapshot.Load() && panelID == h.model.ActivePanel {
		return
	}
	if child.shouldFetch(target, p.CarouselChildCacheValidFor(target), now) {
		h.dispatchCarouselChildSnapshot(panelID, target, h.host.PanelViewportRows(panelID))
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
