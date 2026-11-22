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
func (a *App) initEmbeddedPanel(dst *panel.State) int {
	act := a.activeBrowserPanel()
	st := panel.NewSideListing(act, act)
	id := a.openPanel(dst)
	st.ScheduleAsyncLoad = a.asyncLoadScheduler(id)
	st.ScheduleGitStatus = a.gitStatusScheduler(id)
	st.FileListViewportRows = func() int { return a.panelViewportRows(id) }
	*dst = st
	return id
}

// closeEmbeddedPanel unregisters the embedded panel *dst (registry ID *id), clears it and forgets what
// follow followed.
func (a *App) closeEmbeddedPanel(dst *panel.State, id *int, stale *bool, follow *panelFollow) {
	follow.reset()
	*stale = false
	*dst = panel.State{}
	a.closePanel(*id)
	*id = 0
}

// refreshStaleViewPanel re-lists a shown view panel that skipped refreshes while hidden
// (refreshViewPanels in the dialog handler only marks a hidden one stale).
func (a *App) refreshStaleViewPanel(p *panel.State, id int, stale *bool) {
	if !*stale {
		return
	}
	*stale = false
	_ = p.Refresh(a.panelViewportRows(id))
}

// reconcileDedupPanel keeps the browse panel on the row of the pane whose cursor last moved
// (panelFollow): the directory itself for a dir row, else the file's parent with the cursor on the
// file.
func (a *App) reconcileDedupPanel() {
	if !a.dedupBrowseWanted() {
		if a.dedupBrowseOn {
			a.dedupBrowseOn = false
			a.closeEmbeddedPanel(&a.model.DedupPanel, &a.model.DedupPanelID, &a.model.DedupPanelStale, &a.dedupFollow)
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
		a.model.DedupPanelID = a.initEmbeddedPanel(&a.model.DedupPanel)
	}
	a.refreshStaleViewPanel(&a.model.DedupPanel, a.model.DedupPanelID, &a.model.DedupPanelStale)
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
