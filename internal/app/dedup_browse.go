package app

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// dedupBrowseWanted reports whether the browse panel should exist: kept Dirs-view results. It
// survives leaving the dedup view (focus and navigated location come back on return); only
// losing the results or switching to Groups view tears it down.
func (a *App) dedupBrowseWanted() bool {
	return a.model.DedupView.TreeDirs && a.model.DedupSnapshot.Phase == comparepkg.DedupDone
}

// syntheticPanelActive reports whether an async result for a synthetic panel ID may still land.
// Real panels are always live.
func (a *App) syntheticPanelActive(panelID int) bool {
	switch panelID {
	case ui.QuickViewOverlayPanel:
		return a.model.QuickViewDirOverlayActive
	case ui.DedupBrowsePanel:
		return a.dedupBrowseOn
	}
	return true
}

func (a *App) dedupBrowseRows(layout ui.Layout) int {
	_, browse := ui.DedupSecondaryRects(layout.Secondary, true)
	return ui.PanelListRows(browse)
}

func (a *App) initDedupPanel() {
	act := a.activePanel()
	st := panel.NewSideListing(act, act)
	st.ScheduleAsyncLoad = a.asyncLoadScheduler(ui.DedupBrowsePanel)
	st.ScheduleGitStatus = a.gitStatusScheduler(ui.DedupBrowsePanel)
	st.FileListViewportRows = func() int { return a.panelViewportRows(ui.DedupBrowsePanel) }
	a.model.DedupPanel = st
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
			if a.model.DedupView.FocusPanel {
				a.model.DedupView.FocusPanel = false
				a.model.DedupView.FocusCopies = false // panel gone: fall back to the main tree
			}
			a.panelAsyncLoadGen[ui.DedupBrowsePanel].Add(1)
			a.gitStatusLoadGen[ui.DedupBrowsePanel].Add(1)
		}
		return
	}
	a.dedupBrowseOn = true
	if a.model.ViewMode != ui.ViewDedup {
		return // hidden: keep the panel as-is for the return trip
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
			dirs[i], names[i] = filepath.Dir(path), filepath.Base(path)
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
	if a.model.DedupPanel.ScheduleAsyncLoad == nil {
		a.initDedupPanel()
	}
	if err := a.model.DedupPanel.LoadWithViewport(dir, name, a.panelViewportRows(ui.DedupBrowsePanel), 0); err != nil {
		a.setErrorMessage("Browse", err)
	}
}

// handleDedupPanelKey handles keys while the browse panel has focus: cursor moves, directory
// enter/parent, F3 view and F4 edit. Everything else is swallowed.
func (a *App) handleDedupPanelKey(action string, event *tcell.EventKey) bool {
	p := &a.model.DedupPanel
	rows := a.panelViewportRows(ui.DedupBrowsePanel)
	if action == "" {
		switch event.Key() {
		case tcell.KeyEsc:
			action = keymap.ActionDedupClose
		case tcell.KeyEnter, tcell.KeyRight:
			action = keymap.ActionNavOpen
		case tcell.KeyLeft, tcell.KeyBackspace, tcell.KeyBackspace2:
			action = keymap.ActionNavParent
		}
	}
	switch action {
	case keymap.ActionDedupClose:
		a.closeDedupView()
	case keymap.ActionPanelSwitch:
		a.dedupCtrl.SwitchPane()
	case keymap.ActionNavUp:
		p.Move(-1, rows)
	case keymap.ActionNavDown:
		p.Move(1, rows)
	case keymap.ActionNavPageUp:
		p.Page(-1, rows)
	case keymap.ActionNavPageDown:
		p.Page(1, rows)
	case keymap.ActionNavTop:
		p.Top(rows)
	case keymap.ActionNavBottom:
		p.Bottom(rows)
	case keymap.ActionNavOpen, keymap.ActionDedupToggleNode:
		if _, err := p.Enter(rows); err != nil {
			a.setErrorMessage("Browse", err)
		}
	case keymap.ActionNavParent, keymap.ActionDedupCollapse:
		if err := p.Parent(rows); err != nil {
			a.setErrorMessage("Browse", err)
		}
	case keymap.ActionFileView:
		if e, ok := p.CurrentEntry(); ok && !e.ResolvesToDir() {
			if err := a.previewCtrl.OpenFullscreenFilePreviewAt(e.Path, false); err != nil {
				a.setErrorMessage("View", err)
			}
		}
	case keymap.ActionFileEdit:
		if e, ok := p.CurrentEntry(); ok && !e.ResolvesToDir() {
			if err := a.openFileInExternalEditor(e.Path); err != nil {
				a.setErrorMessage("Edit", err)
			}
		}
	}
	return false
}
