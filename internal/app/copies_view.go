package app

import (
	"github.com/gdamore/tcell/v2"
	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/quickfilter"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// copiesHost adapts *App to the copiesview handler's Host interface.
type copiesHost struct {
	appShellHost
}

func (h copiesHost) NavigatePanelToPath(panelID int, path string, selectName string) error {
	return h.app.navigatePanelToDirectory(panelID, path, selectName)
}

func (h copiesHost) CopiesMenuDefinitions() []menu.Definition { return h.app.copiesMenuDefinitions() }

func (h copiesHost) BrowserMenuDefinitions() []menu.Definition { return h.app.browserMenuDefinitions() }

func (h copiesHost) FilterOptions() quickfilter.Options { return h.app.quickFilterOptions() }

func (a *App) copiesMenuDefinitions() []menu.Definition {
	return menu.CopiesDefinitions(a.keys.Global, a.keys.Copies)
}

// openCopiesView shows the Other copies view and opens its panel on the first copy.
func (a *App) openCopiesView(origin string, indexPanel bool, copies []fi.CopyFile) {
	if ui.IsAuxiliaryView(a.model.ViewMode) {
		return
	}
	a.copiesCtrl.Open(origin, indexPanel, copies)
	a.reconcileCopiesPanel()
}

// closeCopiesView returns to the browser and closes the view's panel.
func (a *App) closeCopiesView() {
	a.copiesCtrl.Leave()
	a.reconcileCopiesPanel()
}

// reconcileCopiesPanel keeps the view's panel in step with the view: it exists while the view is
// open (shown, or hidden behind the fullscreen viewer) and follows the cursor copy while shown (see
// panelFollow).
func (a *App) reconcileCopiesPanel() {
	m := &a.model
	if !m.CopiesView.Open {
		if m.CopiesPanelID != 0 {
			a.closeEmbeddedPanel(&m.CopiesPanel, &m.CopiesPanelID, &a.copiesFollow)
		}
		return
	}
	if m.ViewMode != ui.ViewCopies {
		return // hidden: keep the panel as-is for the return trip
	}
	if m.CopiesPanelID == 0 {
		m.CopiesPanelID = a.initEmbeddedPanel(&m.CopiesPanel, ui.ViewCopies, a.copiesPanelRows)
	}
	a.refreshStaleViewPanel(m.CopiesPanelID)
	path, ok := a.copiesCtrl.Target()
	t, follow := a.copiesFollow.next(0, followTargetOf(path, false, ok))
	if !follow {
		return
	}
	err := a.followLoad(&m.CopiesPanel, m.CopiesPanelID, t)
	if err != nil && !m.CopiesView.IndexPanel {
		// ponytail: the real directory is gone, so show it as the index remembers it
		if loc, ferr := pathloc.FileIndex(t.dir); ferr == nil {
			err = a.followLoad(&m.CopiesPanel, m.CopiesPanelID, followTarget{dir: loc.String(), name: t.name})
		}
	}
	if err != nil {
		a.setErrorMessage("Browse", err)
	}
}

// copiesPanelRows is the number of entry rows the view's panel shows.
func (a *App) copiesPanelRows(layout ui.Layout) int {
	_, rect := ui.CopiesRects(layout)
	return ui.PanelListRows(rect)
}

// copiesVisibleRows is the number of rows the copies list shows.
func (a *App) copiesVisibleRows() int {
	width, height := a.screen.Size()
	list, _ := ui.CopiesRects(a.layoutForTerminalSize(width, height))
	return ui.PanelListRows(list)
}

// openCopiesDeleteDialog opens the standard delete confirmation for the marked copies, else the
// cursor copy, listed by full path (their names are the same).
func (a *App) openCopiesDeleteDialog() {
	a.openListedDeleteDialog(a.copiesCtrl.Sources(), func(path string) string {
		return primitive.PathWithHomeTilde(path, a.model.UserHomeDir)
	}, "")
}

// tryDispatchCopies handles the view's actions from keys, the F9 menu and Help. While the view's
// panel has focus only the view-level actions reach it (tryDispatchEmbeddedPanelView).
func (a *App) tryDispatchCopies(actionID string) bool {
	if a.model.ViewMode != ui.ViewCopies {
		return false
	}
	switch actionID {
	case keymap.ActionCopiesClose:
		a.closeCopiesView()
		return true
	case keymap.ActionPanelClearSelection:
		a.copiesCtrl.ClearMarks()
	case keymap.ActionPanelSelectToggle:
		a.copiesCtrl.SelectToggleAndAdvance()
	case keymap.ActionPanelSwitch, keymap.ActionCopiesPrevPane:
		a.copiesCtrl.SwitchPane()
	case keymap.ActionFileDelete:
		a.openCopiesDeleteDialog()
	case keymap.ActionCopy:
		a.dialogCtrl.ActivateCopyAction()
	case keymap.ActionMove:
		a.dialogCtrl.ActivateMoveAction()
	case keymap.ActionFileView:
		if path, ok := a.copiesCtrl.CursorPath(); !ok {
			a.setTransientMessage("View: no file", ui.MessageUrgencyWarn)
		} else if err := a.previewCtrl.OpenFullscreenFilePreviewAt(path, false); err != nil {
			a.setErrorMessage("View", err)
		}
	case keymap.ActionFileEdit:
		if path, ok := a.copiesCtrl.CursorPath(); !ok {
			a.setTransientMessage("Edit: no file", ui.MessageUrgencyWarn)
		} else if err := a.openFileInExternalEditor(path); err != nil {
			a.setErrorMessage("Edit", err)
		}
	case keymap.ActionOpenInPrimary, keymap.ActionOpenInSecondary:
		if path, ok := a.copiesCtrl.Target(); ok {
			a.copiesCtrl.OpenInPanel(a.openInPanelID(actionID), path, false)
		}
	case keymap.ActionNavUp:
		a.copiesCtrl.MoveSelection(-1)
	case keymap.ActionNavDown:
		a.copiesCtrl.MoveSelection(1)
	case keymap.ActionNavPageUp:
		a.copiesCtrl.MoveSelection(-a.copiesVisibleRows())
	case keymap.ActionNavPageDown:
		a.copiesCtrl.MoveSelection(a.copiesVisibleRows())
	case keymap.ActionNavTop:
		a.copiesCtrl.SelectEdge(false)
	case keymap.ActionNavBottom:
		a.copiesCtrl.SelectEdge(true)
	case keymap.ActionNavOpen:
		if path, ok := a.copiesCtrl.CursorPath(); ok {
			a.openFileExternally(path)
		}
	default:
		return false
	}
	a.copiesCtrl.EnsureSelectionVisible(a.copiesVisibleRows())
	return true
}

func (a *App) handleCopiesViewKey(event *tcell.EventKey) bool {
	// An open type-to-jump filter sees the key before the vi remap so h/j/k/l type into it.
	if f := a.copiesCtrl.Filter(); !a.model.CopiesView.FocusPanel && f.UIActive() {
		if !filterRetainsKey(f, event, a.keys.Global) {
			a.copiesCtrl.CancelFilter()
		} else if a.handleListFilterKey(a.copiesCtrl, event) {
			a.copiesCtrl.EnsureSelectionVisible(a.copiesVisibleRows())
			return false
		}
	}
	if a.model.ViMotionMode {
		event = keymap.RemapViMotionKey(event)
	}
	action := a.actionFromKeyEvent(event)
	if result, handled := a.dispatchAuxiliaryViewCommonKeys(event, action); handled {
		return result
	}
	if action != "" && a.tryDispatchAuxiliaryScreens(action) {
		return false
	}
	if action == "" && keymap.IsPlainPrintableRune(event) {
		a.handleListFilterKey(a.copiesCtrl, event)
		a.copiesCtrl.EnsureSelectionVisible(a.copiesVisibleRows())
		return false
	}
	if action != "" {
		a.tryDispatchCopies(action)
	}
	return false
}

// activateCopiesHelpAction runs a row of the view's Help on the view's own dispatch.
func (a *App) activateCopiesHelpAction(actionID string) bool {
	if handled, quit := a.activateShellHelpAction(actionID); handled {
		return quit
	}
	if !a.tryDispatchCopies(actionID) {
		a.tryDispatchAuxiliaryScreens(actionID)
	}
	return false
}

func copiesViewFooterKeys(global *keymap.Map) []menu.FunctionKey {
	var out []menu.FunctionKey
	if global == nil {
		return out
	}
	for _, f := range []struct{ action, hint string }{
		{keymap.ActionFileView, "View"},
		{keymap.ActionFileEdit, "Edit"},
		{keymap.ActionCopy, "Copy"},
		{keymap.ActionMove, "Move"},
		{keymap.ActionFileDelete, "Delete"},
		{keymap.ActionPanelClearSelection, "Unselect all"},
	} {
		if lbl := global.MenuBindingLabel(f.action); lbl != "" {
			out = append(out, menu.FunctionKey{Key: tcell.KeyNUL, KeyLabel: lbl, Hint: f.hint, ActionID: f.action})
		}
	}
	return out
}

// handleCopiesMouse handles the view's list: a left click focuses it and selects the row, the wheel
// moves the cursor. Off the list, it handles the panel: a left click focuses it and moves its cursor
// to the entry under the pointer, the wheel moves the cursor.
func (a *App) handleCopiesMouse(layout ui.Layout, x, y int, buttons, pressed tcell.ButtonMask) (rendered bool) {
	if row, ok := ui.CopiesListEntryAt(layout, &a.model, x, y); ok {
		return a.handleListViewMouse(a.copiesCtrl, row, a.copiesVisibleRows(), buttons, pressed)
	}
	if row, ok := ui.CopiesPanelEntryAt(layout, &a.model, x, y); ok {
		return a.handleEmbeddedPanelMouse(&a.model.CopiesPanel, a.copiesPanelRows(layout), row, buttons, pressed, a.copiesCtrl.FocusPanel)
	}
	return false
}
