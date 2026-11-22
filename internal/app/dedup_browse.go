package app

import (
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
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

func (a *App) initDedupPanel() {
	act := a.activeBrowserPanel()
	st := panel.NewSideListing(act, act)
	id := a.openPanel(&a.model.DedupPanel)
	st.ScheduleAsyncLoad = a.asyncLoadScheduler(id)
	st.ScheduleGitStatus = a.gitStatusScheduler(id)
	st.FileListViewportRows = func() int { return a.panelViewportRows(id) }
	a.model.DedupPanel = st
	a.model.DedupPanelID = id
}

// reconcileDedupPanel keeps the browse panel on the row of the pane whose cursor last moved: the
// directory itself for a dir row, else the file's parent with the cursor on the file. Only a
// cursor move re-syncs it (switching panes does not); manual navigation inside the panel
// survives until then.
func (a *App) reconcileDedupPanel() {
	if !a.dedupBrowseWanted() {
		if a.dedupBrowseOn {
			a.dedupBrowseOn = false
			a.dedupBrowseFP = [2]string{}
			a.model.DedupPanel = panel.State{}
			a.closePanel(a.model.DedupPanelID)
			a.model.DedupPanelID = 0
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
		a.initDedupPanel() // registered before its first target, so focusing it always finds it
	}
	if a.dedupCtrl.NavHeld() {
		return // fingerprint stays stale so the flush sees the final cursor move
	}
	// Per-pane fingerprints (0 main, 1 copies). Only a cursor move changes one, so switching
	// panes never reloads the panel; the focused pane wins when both moved.
	var fp [2]string
	var dirs, names [2]string
	for i := range fp {
		path, isDir, ok := a.dedupCtrl.PaneTarget(i == 1)
		if !ok || path == "" {
			continue
		}
		dirs[i], names[i] = path, ""
		if !isDir {
			dirs[i], names[i] = pathloc.SplitDirBase(path)
		}
		fp[i] = dirs[i] + "\x00" + names[i]
	}
	prev := a.dedupBrowseFP
	a.dedupBrowseFP = fp
	src := 0
	if a.model.DedupView.FocusCopies {
		src = 1
	}
	pick := -1
	for _, i := range [2]int{src, 1 - src} {
		if fp[i] != "" && fp[i] != prev[i] {
			pick = i
			break
		}
	}
	if pick < 0 {
		return
	}
	dir, name := dirs[pick], names[pick]
	if err := a.model.DedupPanel.LoadWithViewport(dir, name, a.panelViewportRows(a.model.DedupPanelID), 0); err != nil {
		a.setErrorMessage("Browse", err)
	}
}
