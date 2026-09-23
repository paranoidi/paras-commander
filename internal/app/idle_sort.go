package app

import (
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// idleSortSpec parameterizes the shared "wait for idle input, then re-sort" machinery behind
// both disk-usage and meta-column idle sort (see disk_usage.go's diskIdleSortSpec and
// meta_idle_sort.go's metaIdleSortSpec): which per-panel timer/epoch slots and last-reconciled
// nav-path bookkeeping to use, which idleSortPayload Kind to post, and the mechanism-specific
// predicates. eligible is the coarse "is this turned on for the panel at all" gate (view mode,
// sort mode/toggle); resolved is "is the data behind it ready to sort by right now"; apply
// performs the actual sort.
type idleSortSpec struct {
	kind        idleSortKind
	slots       *[2]diskIdleSortPanel
	idleNavPath *[2]string
	eligible    func(panelID int) bool
	resolved    func(panelID int) bool
	apply       func(panelID int)
}

func (a *App) idleSortReady(s idleSortSpec, panelID int) bool {
	return s.eligible(panelID) && s.resolved(panelID)
}

// idleSortMaybeSchedule is the common entry point for "something changed, maybe (re)start the
// idle timer" call sites (a column/scan just resolved, a directory-change reconcile, user
// activity). The readiness gate lives once, in idleSortArm, so this never re-derives it itself.
func (a *App) idleSortMaybeSchedule(s idleSortSpec, panelID int) {
	a.idleSortArm(s, panelID)
}

// idleSortArm (re)starts panelID's idle timer, or leaves it stopped when not ready. Safe to call
// directly (tests do, to force-arm without going through maybeSchedule's callers): it always
// stops any existing timer first, then re-arms only if eligible and resolved.
func (a *App) idleSortArm(s idleSortSpec, panelID int) {
	if panelID != ui.PrimaryPanel && panelID != ui.SecondaryPanel {
		return
	}
	ps := &s.slots[panelID]
	if ps.timer != nil {
		ps.timer.Stop()
		ps.timer = nil
	}
	if !a.idleSortReady(s, panelID) {
		return
	}
	delayMS := a.config.DiskUsage.IdleSortDelayMS
	if delayMS <= 0 {
		delayMS = config.DefaultDiskUsageIdleSortDelayMS
	}
	delay := time.Duration(delayMS) * time.Millisecond
	epochSnap := ps.epoch
	kind := s.kind
	pid := panelID
	// ponytail: don't clear ps.timer here — this callback runs on the timer goroutine while
	// the event loop reads/writes ps.timer, and a Stop() on an already-fired timer is a
	// harmless no-op. The epoch check in idleSortApplyPayload discards stale payloads instead.
	ps.timer = time.AfterFunc(delay, func() {
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(idleSortPayload{Kind: kind, PanelID: pid, Epoch: epochSnap}))
	})
}

// idleSortApplyPayload applies the deferred re-sort once an idleSortPayload's timer fires,
// dropping it when the epoch is stale (invalidated since the timer was armed) or the panel is no
// longer eligible/resolved.
func (a *App) idleSortApplyPayload(s idleSortSpec, panelID int, epoch uint64) {
	if panelID != ui.PrimaryPanel && panelID != ui.SecondaryPanel {
		return
	}
	ps := &s.slots[panelID]
	if ps.epoch != epoch {
		return
	}
	if !a.idleSortReady(s, panelID) {
		return
	}
	s.apply(panelID)
}

func (a *App) idleSortInvalidate(s idleSortSpec, panelID int) {
	if panelID != ui.PrimaryPanel && panelID != ui.SecondaryPanel {
		return
	}
	ps := &s.slots[panelID]
	if ps.timer != nil {
		ps.timer.Stop()
		ps.timer = nil
	}
	ps.epoch++
}

func (a *App) idleSortInvalidateBoth(s idleSortSpec) {
	a.idleSortInvalidate(s, ui.PrimaryPanel)
	a.idleSortInvalidate(s, ui.SecondaryPanel)
}

func (a *App) idleSortDeferOnUserActivity(s idleSortSpec) {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	a.idleSortMaybeSchedule(s, ui.PrimaryPanel)
	a.idleSortMaybeSchedule(s, ui.SecondaryPanel)
}

// idleSortDirChanged invalidates any pending timer for panelID when its directory changed since
// the last reconcile, so a stale timer never fires a re-sort meant for the directory the user
// just left. Early-returns when not eligible, before computing the path.
func (a *App) idleSortDirChanged(s idleSortSpec, panelID int) {
	if !s.eligible(panelID) {
		return
	}
	p := a.panelByID(panelID)
	cur := filepath.Clean(p.PathString())
	if s.idleNavPath[panelID] != cur {
		a.idleSortInvalidate(s, panelID)
		s.idleNavPath[panelID] = cur
	}
}
