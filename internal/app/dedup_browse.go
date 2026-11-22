package app

import (
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// dedupBrowseWanted reports whether the browse panel should exist: kept dedup results. It
// survives leaving the dedup view (focus and navigated location come back on return); only
// losing the results tears it down.
func (a *App) dedupBrowseWanted() bool {
	return a.model.DedupSnapshot.Phase == comparepkg.DedupDone
}

func (a *App) dedupBrowseRows(layout ui.Layout) int {
	_, browse := ui.DedupSecondaryRects(layout.Secondary)
	return ui.PanelListRows(browse)
}

// initEmbeddedPanel registers *dst as a new file-list panel instance listing what the active browser
// panel lists, and returns its ID. It is registered before its first target, so focusing it always
// finds it.
// view is the view that shows it and rows its viewport rows (panelViewportRows).
func (a *App) initEmbeddedPanel(dst *panel.State, view ui.ViewMode, rows func(ui.Layout) int) int {
	act := a.activeBrowserPanel()
	st := panel.NewSideListing(act, act)
	id := a.openPanel(dst)
	inst := a.panelInst(id)
	inst.rows, inst.view = rows, view
	st.ScheduleAsyncLoad = a.asyncLoadScheduler(id)
	st.ScheduleGitStatus = a.gitStatusScheduler(id)
	st.FileListViewportRows = func() int { return a.panelViewportRows(id) }
	*dst = st
	return id
}

// closeEmbeddedPanel unregisters the embedded panel *dst (registry ID *id), clears it and forgets what
// follow followed.
func (a *App) closeEmbeddedPanel(dst *panel.State, id *int, follow *panelFollow) {
	follow.reset()
	*dst = panel.State{}
	a.closePanel(*id)
	*id = 0
}

// refreshStaleViewPanel re-lists a shown view panel that skipped refreshes while hidden
// (refreshEmbeddedPanels only marks a hidden one stale).
func (a *App) refreshStaleViewPanel(id int) {
	inst := a.panelInst(id)
	if inst == nil || !inst.stale {
		return
	}
	inst.stale = false
	_ = inst.state.Refresh(a.panelViewportRows(id))
}

// refreshEmbeddedPanels re-lists every open view panel whose view is shown, with the reload hook
// onApplied(id) returns (nil for none); file operations run from them (and jobs from anywhere)
// change what they list, as they do for the browser panels. A panel whose view is hidden is only
// marked stale (re-listing it is wasted work if the view is never shown again); the view's
// reconcile re-lists it on return.
func (a *App) refreshEmbeddedPanels(onApplied func(id int) func()) {
	for id, inst := range a.panels.byID {
		switch {
		case inst.rows == nil:
		case inst.view == a.model.ViewMode:
			_ = inst.state.RefreshWithHook(a.panelViewportRows(id), onApplied(id))
		default:
			inst.stale = true
		}
	}
}

// isEmbeddedPanel reports whether id is an open view panel.
func (a *App) isEmbeddedPanel(id int) bool {
	inst := a.panelInst(id)
	return inst != nil && inst.rows != nil
}

// reconcileDedupPanel keeps the browse panel on the row of the pane whose cursor last moved
// (panelFollow): the directory itself for a dir row, else the file's parent with the cursor on the
// file.
func (a *App) reconcileDedupPanel() {
	if !a.dedupBrowseWanted() {
		if a.dedupBrowseOn {
			a.dedupBrowseOn = false
			a.closeEmbeddedPanel(&a.model.DedupPanel, &a.model.DedupPanelID, &a.dedupFollow)
			if a.model.DedupView.FocusPanel {
				a.model.DedupView.FocusPanel = false
				a.model.DedupView.FocusCopies = false // panel gone: fall back to the main tree
			}
		}
		return
	}
	a.dedupBrowseOn = true
	if a.model.ViewMode != ui.ViewDedup {
		return // hidden: keep the panel as-is for the return trip
	}
	if a.model.DedupPanelID == 0 {
		a.model.DedupPanelID = a.initEmbeddedPanel(&a.model.DedupPanel, ui.ViewDedup, a.dedupBrowseRows)
	}
	a.refreshStaleViewPanel(a.model.DedupPanelID)
	if a.dedupCtrl.NavHeld() {
		return // fingerprint stays stale so the flush sees the final cursor move
	}
	// Per-pane targets (0 main, 1 copies); the focused pane wins when both moved.
	src := 0
	if a.model.DedupView.FocusCopies {
		src = 1
	}
	var targets [2]followTarget
	for i := range targets {
		path, isDir, ok := a.dedupCtrl.PaneTarget(i == 1)
		targets[i] = followTargetOf(path, isDir, ok)
	}
	t, ok := a.dedupFollow.next(src, targets[:]...)
	if !ok {
		return
	}
	if err := a.followLoad(&a.model.DedupPanel, a.model.DedupPanelID, t); err != nil {
		a.setErrorMessage("Browse", err)
	}
}
