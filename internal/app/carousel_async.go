package app

import (
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/sched"
)

// carouselPaintDeferMaxMS bounds how long a repaint may be held back waiting for the carousel
// parent-column snapshot that determines column geometry (see renderAfterAsyncApply). A local
// listing lands in a millisecond or two, so in normal use the deferred paint is released by the
// snapshot itself and the new directory is drawn exactly once. This deadline only matters when
// that fetch is slow — precisely the contended-volume case this async path exists for — where
// leaving navigation visually frozen until the fetch (or its list_timeout_secs give-up) returns
// would be far worse than briefly painting against the previous parent's measurements.
const carouselPaintDeferMaxMS = 120

// carouselPaintReleasePayload releases a deferred carousel repaint whose parent snapshot did not
// arrive within carouselPaintDeferMaxMS.
type carouselPaintReleasePayload struct {
	panelID int
	epoch   uint64
}

// carouselPaintDeferState is one panel's deferred-repaint bookkeeping: whether a paint is
// currently held back, the deadline that force-releases it, and the epoch that makes a superseded
// deadline a no-op once its event reaches the main loop.
type carouselPaintDeferState struct {
	active   bool
	deadline sched.Debouncer
	epoch    uint64
}

// armCarouselPaintDeferTimer starts (or restarts) the deadline that force-releases panelID's
// deferred repaint.
func (a *App) armCarouselPaintDeferTimer(panelID int) {
	d := &a.carouselPaintDefer[panelID]
	d.epoch++
	epoch := d.epoch
	d.deadline.Arm(carouselPaintDeferMaxMS*time.Millisecond, func() {
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(carouselPaintReleasePayload{panelID: panelID, epoch: epoch}))
	})
}

// clearCarouselPaintDefer drops panelID's deferred-paint flag and cancels its deadline, bumping
// the epoch so a deadline already in flight becomes a no-op when its event lands.
func (a *App) clearCarouselPaintDefer(panelID int) {
	d := &a.carouselPaintDefer[panelID]
	d.active = false
	d.deadline.Invalidate()
	d.epoch++
}

// applyCarouselSnapshotAndRender applies a landed side-column snapshot and repaints when it
// changed anything OR a paint was deferred waiting on it — the latter even if the fetch failed or
// timed out, so a held-back paint is never stranded. Reports whether it rendered.
//
// A deferred paint waits on the PARENT column specifically (it alone drives column geometry), so a
// child snapshot landing first must not release it: both columns are dispatched in the same
// reconcile pass and race, and a small child directory routinely beats a large parent one. Painting
// on the child's arrival would produce exactly the two-phase relayout the deferral exists to avoid.
// The carouselPaintDeferMaxMS deadline still guarantees release if the parent fetch never lands.
func (a *App) applyCarouselSnapshotAndRender(d carouselSnapshotPayload) bool {
	applied := a.applyCarouselSnapshot(d)
	deferred := a.carouselPaintDefer[d.panelID].active
	if deferred && a.carouselParentPaintPending(d.panelID) {
		return false
	}
	a.clearCarouselPaintDefer(d.panelID)
	if !applied && !deferred {
		return false
	}
	a.renderPanelAsyncResult(d.panelID)
	return true
}

// applyCarouselPaintRelease repaints panelID when its deferred paint is still outstanding and the
// deadline that fired is the current one.
func (a *App) applyCarouselPaintRelease(p carouselPaintReleasePayload) bool {
	if a.carouselPaintDefer[p.panelID].epoch != p.epoch || !a.carouselPaintDefer[p.panelID].active {
		return false
	}
	a.clearCarouselPaintDefer(p.panelID)
	a.renderPanelAsyncResult(p.panelID)
	return true
}

// carouselSnapshotPayload delivers an async carousel side-column listing fetch back to the main
// loop. isChild distinguishes the child column from the parent column within panelID.
// internal/panel.State.SnapshotParent/SnapshotChild only ever read the resulting cache — they
// never touch the filesystem — so painting the carousel never blocks the UI on I/O.
type carouselSnapshotPayload struct {
	panelID        int
	isChild        bool
	gen            uint64
	target         string // cache key: CarouselParentPreviewTarget/CarouselChildPreviewTarget
	loc            pathloc.Path
	entries        []fsbackend.Entry
	selectedName   string
	indexFallback  int
	viewportRows   int
	centerRecalled bool
	err            error
}

// scheduleCarouselParentSnapshot dispatches an async fetch of panelID's carousel parent-column
// preview (the parent of the panel's current directory) off the UI thread.
func (a *App) scheduleCarouselParentSnapshot(panelID int, viewportRows int) {
	pan := a.panelByID(panelID)
	if pan == nil || pan.Path.IsZero() {
		return
	}
	parent := pan.Path.Parent()
	if parent.Equal(pan.Path) {
		return
	}
	target, ok := pan.CarouselParentPreviewTarget()
	if !ok {
		return
	}
	a.scheduleCarouselSnapshot(panelID, false, parent, target, pan.Path.Base(), panel.NoIndexCursorFallback, false, viewportRows)
}

// scheduleCarouselChildSnapshot dispatches an async fetch of panelID's carousel child-column
// preview (the directory under the cursor) off the UI thread.
func (a *App) scheduleCarouselChildSnapshot(panelID int, viewportRows int) {
	pan := a.panelByID(panelID)
	if pan == nil {
		return
	}
	target, ok := pan.CarouselChildPreviewTarget()
	if !ok {
		return
	}
	child, err := pathloc.Parse(target)
	if err != nil {
		return
	}
	selectedName, indexFallback, centerRecalled := pan.RecalledCursorFor(target)
	a.scheduleCarouselSnapshot(panelID, true, child, target, selectedName, indexFallback, centerRecalled, viewportRows)
}

func (a *App) carouselSnapshotGenSlot(panelID int, isChild bool) *atomic.Uint64 {
	if isChild {
		return &a.carouselChildSnapshotGen[panelID]
	}
	return &a.carouselParentSnapshotGen[panelID]
}

func (a *App) scheduleCarouselSnapshot(panelID int, isChild bool, loc pathloc.Path, target, selectedName string, indexFallback int, centerRecalled bool, viewportRows int) {
	gen := a.carouselSnapshotGenSlot(panelID, isChild).Add(1)
	timeout := time.Duration(a.config.SFTP.ListTimeoutSecs) * time.Second
	snap := a.panelByID(panelID).ListingRefreshSnapshot(loc, timeout)
	a.raceAsyncListingFetch(snap, timeout, func(resolvedLoc pathloc.Path, entries []fsbackend.Entry, _, _ bool, err error) {
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(carouselSnapshotPayload{
			panelID:        panelID,
			isChild:        isChild,
			gen:            gen,
			target:         target,
			loc:            resolvedLoc,
			entries:        entries,
			selectedName:   selectedName,
			indexFallback:  indexFallback,
			viewportRows:   viewportRows,
			centerRecalled: centerRecalled,
			err:            err,
		}))
	})
}

// applyCarouselSnapshot applies an async carousel side-column fetch result into CarouselSideCache,
// dropping it if a newer fetch for the same panel/column has since been scheduled or the fetch
// failed (including a timeout) — the render path simply keeps showing the last valid cache.
func (a *App) applyCarouselSnapshot(p carouselSnapshotPayload) bool {
	if a.carouselSnapshotGenSlot(p.panelID, p.isChild).Load() != p.gen {
		return false
	}
	if p.err != nil {
		// Let the preview handler retry this target later. Without this the fetch is never
		// re-attempted while the panel stays put: its in-flight marker still names the target and
		// the cache never becomes valid, so neither half of the reconcile dispatch condition can be
		// satisfied again. The parent column would keep painting whichever directory was cached
		// last — SnapshotParent returns the cache without checking which directory it holds.
		a.previewCtrl.NoteCarouselSnapshotFailed(p.panelID, p.isChild, p.target)
		return false
	}
	pan := a.panelByID(p.panelID)
	if pan == nil {
		return false
	}
	snap, err := pan.BuildListingSnapshotFromEntries(p.loc, p.entries, p.selectedName, p.indexFallback, p.viewportRows, p.centerRecalled)
	if err != nil {
		return false
	}
	if p.isChild {
		pan.CarouselSideCache.Child = snap
		pan.CarouselSideCache.ChildOK = true
		pan.CarouselSideCache.ChildCursorDir = p.target
	} else {
		pan.CarouselSideCache.Parent = snap
		pan.CarouselSideCache.ParentOK = true
		pan.CarouselSideCache.ParentSourceDir = p.target
	}
	return true
}
