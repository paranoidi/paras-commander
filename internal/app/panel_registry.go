package app

import (
	"sync/atomic"
	"time"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/sched"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// panelInstance is one file-list panel: its panel.State plus every piece of per-panel runtime
// state the app keeps beside it (load generations, coalescing queues, refresh bookkeeping).
//
// Concurrency: the registry (App.panels) is touched on the main event loop goroutine only.
// Goroutines (listing, git status, refresh, statfs, index drift, carousel) capture the
// *panelInstance at schedule time and use its atomics, queues and mutexes; they never call
// panelInst.
type panelInstance struct {
	state *panel.State

	// panelAsyncLoadGen / gitStatusLoadGen drop a listing / cwd-level git result superseded by a
	// newer dispatch (or by closePanel).
	panelAsyncLoadGen atomic.Uint64
	gitStatusLoadGen  atomic.Uint64
	asyncQ            coalescingQueue[panelAsyncLoadPayload]
	carouselQ         coalescingQueue[carouselSnapshotPayload]

	// carouselParentSnapshotGen/carouselChildSnapshotGen guard async carousel side-column
	// snapshot fetches against a superseded result landing late (carousel_async.go).
	carouselParentSnapshotGen atomic.Uint64
	carouselChildSnapshotGen  atomic.Uint64
	// carouselPaintDefer holds back a repaint while the carousel parent-column snapshot for the
	// newly navigated-to directory has not landed yet (carousel_async.go).
	carouselPaintDefer carouselPaintDeferState

	// drift is the cancel and directory of the fileindex drift passes (index_overlay.go);
	// explicitDrift records the explicit drift requests and results of the current listing.
	drift         indexDriftState
	explicitDrift explicitDriftNames

	// diskIdle / metaIdle are the idle re-sort timers (idle_sort.go).
	diskIdle diskIdleSortPanel
	metaIdle diskIdleSortPanel

	// connectGen tags SFTP connect attempts (sftp.go).
	connectGen atomic.Uint64

	// selectionSize* skip reconcile work when selection-derived input is unchanged
	// (selection_size.go); the Debouncer defers the mount-exclusion stat check off the main loop.
	selectionSizeScanFP       string
	selectionSizeScanGen      uint64
	selectionSizeScanPath     string
	selectionSizeScanDebounce sched.Debouncer

	// volumeRefreshInFlight / panelRefreshInFlight guard the statfs and periodic listing refresh
	// goroutines; panelRefreshNotBefore is the unix-nanos earliest start of the next periodic
	// refresh (written by the refresh goroutine, read on the main goroutine).
	volumeRefreshInFlight atomic.Bool
	panelRefreshInFlight  atomic.Bool
	panelRefreshNotBefore atomic.Int64

	// dirWatchSig is the signature the current directory watch set was computed from.
	dirWatchSig dirWatchSig

	// dirLoadIndicatorTimer/Epoch arm the row working-indicator icon (dir_loading_indicator.go).
	dirLoadIndicatorTimer *time.Timer
	dirLoadIndicatorEpoch uint64
}

// panelRegistry maps panel IDs to live instances. IDs 0 and 1 (ui.PrimaryPanel, ui.SecondaryPanel)
// are registered at startup and never closed; every later ID is fresh and never reused within a
// process, so a result carrying the ID of a closed panel can never land on a newer one.
type panelRegistry struct {
	byID map[int]*panelInstance
	next int
}

// openPanel registers state as a new panel instance and returns its ID. Main goroutine only.
func (a *App) openPanel(state *panel.State) int {
	if a.panels.byID == nil {
		a.panels.byID = map[int]*panelInstance{}
	}
	id := a.panels.next
	a.panels.next++
	a.panels.byID[id] = &panelInstance{state: state}
	return id
}

// closePanel retires panel id: in-flight loads and git results are superseded (generations
// bumped), the drift context is cancelled, and the ID stops resolving. Unknown IDs and the two
// browser panels (ui.PrimaryPanel, ui.SecondaryPanel) are a no-op.
func (a *App) closePanel(id int) {
	inst := a.panelInst(id)
	if inst == nil || ui.IsBrowserPanel(id) {
		return
	}
	inst.panelAsyncLoadGen.Add(1)
	inst.gitStatusLoadGen.Add(1)
	inst.drift.mu.Lock()
	if inst.drift.cancel != nil {
		inst.drift.cancel()
	}
	inst.drift.mu.Unlock()
	delete(a.panels.byID, id)
}

// panelInst returns the live instance for id, or nil once it is closed or if it never existed.
// An ID is live iff registered. Main goroutine only.
func (a *App) panelInst(id int) *panelInstance {
	return a.panels.byID[id]
}
