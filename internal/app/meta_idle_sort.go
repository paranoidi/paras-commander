package app

import (
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// metaIdleSortState groups meta-column idle re-sort bookkeeping, one slot per panel. Same
// timer/epoch shape as disk usage's idle sort (diskIdleSortPanel) — both mechanisms share the
// idleSortSpec machinery in idle_sort.go, gated on a resolved meta column instead of a fully
// disk-cached listing.
type metaIdleSortState struct {
	idleSort [2]diskIdleSortPanel
	// idleNavPath records the last panel path reconciled by handleMetaIdleSortPanelDirChanged,
	// so a directory change invalidates any pending timer even when the new directory's meta
	// column happens to already read as resolved.
	idleNavPath [2]string
}

// metaIdleSortEligible reports whether panelID is currently sorted by an active meta column at
// all (the coarse gate; metaCtrl.ColumnResolved decides whether it's actually ready to re-sort).
func (a *App) metaIdleSortEligible(p *panel.State) bool {
	return a.model.ViewMode == ui.ViewBrowser && p.Sort.Mode == panel.SortMeta && p.Sort.MetaColumn != ""
}

// metaIdleSortSpec builds the idleSortSpec for meta-column idle sort: eligible is
// metaIdleSortEligible, resolved is metaCtrl.ColumnResolved (every cell dispatched for the
// sorted-on column has a real value, no longer the running-marker), and apply re-sorts the panel
// by its current sort state.
func (a *App) metaIdleSortSpec() idleSortSpec {
	return idleSortSpec{
		kind:        idleSortKindMeta,
		slots:       &a.metaSort.idleSort,
		idleNavPath: &a.metaSort.idleNavPath,
		eligible: func(panelID int) bool {
			return a.metaIdleSortEligible(a.panelByID(panelID))
		},
		resolved: a.metaCtrl.ColumnResolved,
		apply: func(panelID int) {
			p := a.panelByID(panelID)
			p.ApplySortFromDialog(p.Sort, a.panelViewportRows(panelID))
		},
	}
}

// maybeScheduleMetaIdleSort arms the idle re-sort timer for panelID when eligible and resolved.
// Called both when the meta handler signals a column just finished (NoteMetaColumnResolved) and,
// to keep pushing the timer out, on every user key press (deferMetaIdleSortOnUserActivity).
func (a *App) maybeScheduleMetaIdleSort(panelID int) {
	a.idleSortMaybeSchedule(a.metaIdleSortSpec(), panelID)
}

func (a *App) applyMetaIdleSort(panelID int, epoch uint64) {
	a.idleSortApplyPayload(a.metaIdleSortSpec(), panelID, epoch)
}

func (a *App) invalidateMetaIdleSortPanel(panelID int) {
	a.idleSortInvalidate(a.metaIdleSortSpec(), panelID)
}

func (a *App) invalidateMetaIdleSortBothPanels() {
	a.idleSortInvalidateBoth(a.metaIdleSortSpec())
}

// deferMetaIdleSortOnUserActivity is called on every key press (see handleKey): re-arming via
// maybeScheduleMetaIdleSort restarts the idle-delay timer, so continuous activity keeps pushing
// the re-sort out instead of firing mid-interaction.
func (a *App) deferMetaIdleSortOnUserActivity() {
	a.idleSortDeferOnUserActivity(a.metaIdleSortSpec())
}

// handleMetaIdleSortPanelDirChanged invalidates any pending meta idle-sort timer for panelID
// when its directory changed since the last reconcile, so a stale timer never fires a re-sort
// meant for the directory the user just left. Called from reconcileAfterEvent for both panels
// every Run-loop iteration, mirroring handlePanelDirChanged's disk-usage equivalent.
func (a *App) handleMetaIdleSortPanelDirChanged(panelID int) {
	a.idleSortDirChanged(a.metaIdleSortSpec(), panelID)
}
