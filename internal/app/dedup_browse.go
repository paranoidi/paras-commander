package app

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// dedupBrowseWanted reports whether the browse panel should exist: kept dedup results. It
// survives leaving the dedup view (focus and navigated location come back on return); only
// losing the results tears it down.
func (a *App) dedupBrowseWanted() bool {
	return a.model.DedupSnapshot.Phase == comparepkg.DedupDone
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
	_, browse := ui.DedupSecondaryRects(layout.Secondary)
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
		action = auxPanelRawKeyAction(event)
	}
	switch action {
	case keymap.ActionDedupClose, auxPanelRawKeyClose:
		a.closeDedupView()
	case keymap.ActionPanelSwitch:
		a.dedupCtrl.SwitchPane()
	case keymap.ActionDedupPrevPane:
		a.dedupCtrl.SwitchPanePrev()
	case keymap.ActionDedupToggleNode:
		a.handleAuxPanelNavKey(p, rows, keymap.ActionNavOpen)
	case keymap.ActionDedupCollapse:
		a.handleAuxPanelNavKey(p, rows, keymap.ActionNavParent)
	case keymap.ActionOpenInPrimary, keymap.ActionOpenInSecondary:
		if e, ok := p.CurrentEntry(); ok {
			a.dedupCtrl.OpenInPanel(a.openInPanelID(action), e.Path, e.ResolvesToDir())
		}
	default:
		a.handleAuxPanelNavKey(p, rows, action)
	}
	return false
}

// auxPanelRawKeyClose is what auxPanelRawKeyAction returns for Esc: each view closes itself.
const auxPanelRawKeyClose = "esc"

// auxPanelRawKeyAction maps an unbound key of a panel inside a tree view to its action.
func auxPanelRawKeyAction(event *tcell.EventKey) string {
	switch event.Key() {
	case tcell.KeyEsc:
		return auxPanelRawKeyClose
	case tcell.KeyEnter, tcell.KeyRight:
		return keymap.ActionNavOpen
	case tcell.KeyLeft, tcell.KeyBackspace, tcell.KeyBackspace2:
		return keymap.ActionNavParent
	}
	return ""
}

// handleAuxPanelNavKey runs the browsing actions of a panel inside a tree view (the duplicates
// view's browse panel, the unique files view's destination): cursor moves, directory enter and
// parent, history, view, edit. Anything else is swallowed.
func (a *App) handleAuxPanelNavKey(p *panel.State, rows int, action string) {
	switch action {
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
	case keymap.ActionNavOpen:
		entered, err := p.Enter(rows)
		if err != nil {
			a.setErrorMessage("Browse", err)
		} else if !entered {
			if e, ok := p.CurrentEntry(); ok && !e.ResolvesToDir() {
				a.openFileExternally(e.Path)
			}
		}
	case keymap.ActionPanelExternalBrowser:
		a.openDirInExternalBrowser(p.PathString())
	case keymap.ActionNavParent:
		if err := p.Parent(rows); err != nil {
			a.setErrorMessage("Browse", err)
		}
	case keymap.ActionNavForward, keymap.ActionNavBackward:
		a.stepPanelHistory(p, action, rows)
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
}
