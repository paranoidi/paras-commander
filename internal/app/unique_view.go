package app

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// uniqueDeleteNote is the delete confirmation's warning: this view lists what has no other copy.
const uniqueDeleteNote = "The index knows no other copy of these files: deleting them may be final."

// uniqueHost adapts *App to the uniquefiles handler's Host interface.
type uniqueHost struct {
	appShellHost
}

func (h uniqueHost) NavigatePanelToPath(panelID int, path string, selectName string) error {
	return h.app.navigatePanelToDirectory(panelID, path, selectName)
}

func (h uniqueHost) PanelByID(panelID int) *panel.State { return h.app.panelByID(panelID) }

func (h uniqueHost) AddTaskJob(task *jobs.Task, sources []pathloc.Path) {
	h.app.jobsCtrl.AddTaskJob(task, sources)
}

// EnqueueDeleteJob deletes from disk. The view has asked about empty directories already, so the
// generic dangling-directories cleanup must not ask again, as in the duplicates view.
func (h uniqueHost) EnqueueDeleteJob(paths []string, removeEmptyDirs bool) {
	h.app.jobsCtrl.EnqueueDedupDeleteJob(paths, removeEmptyDirs)
}

func (h uniqueHost) UniqueMenuDefinitions() []menu.Definition { return h.app.uniqueMenuDefinitions() }

func (h uniqueHost) BrowserMenuDefinitions() []menu.Definition { return h.app.browserMenuDefinitions() }

func (h uniqueHost) Config() config.Config { return h.app.config }

// closeUniqueView leaves the view; its results stay for the action that opened it.
func (a *App) closeUniqueView() {
	a.uniqueCtrl.Leave()
	if lbl := a.keys.Global.MenuBindingLabel(keymap.ActionFileIndexFindUnique); lbl != "" {
		a.setTransientMessage("Unique files kept — "+lbl+" returns", ui.MessageUrgencyInfo)
	}
}

// uniqueVisibleRows is the number of rows the results tree shows.
func (a *App) uniqueVisibleRows() int {
	width, height := a.screen.Size()
	tree, _ := ui.UniqueRects(a.layoutForTerminalSize(width, height), a.model.ActivePanel)
	return ui.PanelListRows(tree)
}

// openUniqueDeleteDialog opens the standard delete confirmation for the view's files, with what
// makes this delete different stated in its note.
func (a *App) openUniqueDeleteDialog() {
	files := a.uniqueCtrl.Sources()
	if len(files) == 0 {
		a.setTransientMessage("No files to delete", ui.MessageUrgencyInfo)
		return
	}
	entries := make([]dialog.DeleteListEntry, len(files))
	var bytes int64
	for i, f := range files {
		bytes += f.Size
		entries[i] = dialog.DeleteListEntry{Name: a.model.UniqueView.RelPath(f.Path), Path: f.Path, Type: localfs.EntryFile}
	}
	fd := dialog.FileDialogState{
		Open:          true,
		DialogType:    dialog.FileDialogDelete,
		DeleteSummary: ui.FormatDeleteImpactSummary(int64(len(files)), bytes, false, a.styles.IconWorking()),
		DeleteEntries: entries,
		Note:          uniqueDeleteNote,
		Focus:         dlg.Cancel, // No is the safe default
	}
	fd.DeleteLayoutMinWidth = dialog.ComputeDeleteDialogLayoutMinWidth(fd, ui.DialogListIconLeadingWidth(a.model.UseNerdfontIcons))
	a.model.FileDialog = fd
}

// tryDispatchTreeViews routes an action to whichever results-tree view is open.
func (a *App) tryDispatchTreeViews(actionID string) bool {
	return a.tryDispatchUnique(actionID) || a.tryDispatchDedup(actionID)
}

// tryDispatchUnique handles the view's actions from keys, the F9 menu and Help.
func (a *App) tryDispatchUnique(actionID string) bool {
	if a.model.ViewMode != ui.ViewUnique {
		return false
	}
	switch actionID {
	case keymap.ActionUniqueClose, keymap.ActionFileIndexFindUnique:
		a.closeUniqueView()
	case keymap.ActionPanelRefresh:
		a.uniqueCtrl.Refresh()
	case keymap.ActionPanelToggleHidden:
		a.uniqueCtrl.ToggleHidden()
	case keymap.ActionUniqueCollapseAll:
		a.uniqueCtrl.CollapseAll()
	case keymap.ActionUniqueExpandAll:
		a.uniqueCtrl.ExpandAll()
	case keymap.ActionUniquePrevDir:
		a.uniqueCtrl.MoveToAdjacentDir(-1)
	case keymap.ActionUniqueNextDir:
		a.uniqueCtrl.MoveToAdjacentDir(1)
	case keymap.ActionUniqueToggleNode:
		a.uniqueCtrl.ExpandOrDescend()
	case keymap.ActionUniqueCollapse:
		a.uniqueCtrl.CollapseOrParent()
	case keymap.ActionUniqueHashUnverified:
		a.uniqueCtrl.HashUnverified()
	case keymap.ActionPanelClearSelection:
		a.uniqueCtrl.ClearMarks()
	case keymap.ActionPanelSelectRange:
		res, added := a.uniqueCtrl.SelectRange()
		a.announceRange(res, added)
	case keymap.ActionPanelSelectToggle:
		a.uniqueCtrl.SelectToggleAndAdvance()
	case keymap.ActionPanelSwitch, keymap.ActionUniquePrevPane:
		a.uniqueCtrl.SwitchPane()
	case keymap.ActionFileDelete:
		a.openUniqueDeleteDialog()
	case keymap.ActionCopy:
		a.dialogCtrl.ActivateCopyAction()
	case keymap.ActionMove:
		a.dialogCtrl.ActivateMoveAction()
	case keymap.ActionFileView:
		if path, ok := a.uniqueCtrl.OpenableFile(); !ok {
			a.setTransientMessage("View: not a file", ui.MessageUrgencyWarn)
		} else if err := a.previewCtrl.OpenFullscreenFilePreviewAt(path, false); err != nil {
			a.setErrorMessage("View", err)
		}
	case keymap.ActionFileEdit:
		if path, ok := a.uniqueCtrl.OpenableFile(); !ok {
			a.setTransientMessage("Edit: not a file", ui.MessageUrgencyWarn)
		} else if err := a.openFileInExternalEditor(path); err != nil {
			a.setErrorMessage("Edit", err)
		}
	case keymap.ActionPanelExternalBrowser:
		if path, isDir, ok := a.uniqueCtrl.Target(); ok {
			if !isDir {
				path = filepath.Dir(path)
			}
			a.openDirInExternalBrowser(path)
		}
	case keymap.ActionPanelPinToggle:
		if path, isDir, ok := a.uniqueCtrl.Target(); ok {
			a.pinCtrl.TogglePath(filepath.Base(path), path, isDir)
		}
	case keymap.ActionOpenInPrimary, keymap.ActionOpenInSecondary:
		if path, isDir, ok := a.uniqueCtrl.Target(); ok {
			a.uniqueCtrl.OpenInPanel(a.openInPanelID(actionID), path, isDir)
		}
	case keymap.ActionNavUp:
		a.uniqueCtrl.MoveSelection(-1)
	case keymap.ActionNavDown:
		a.uniqueCtrl.MoveSelection(1)
	case keymap.ActionNavPageUp:
		a.uniqueCtrl.MoveSelection(-a.uniqueVisibleRows())
	case keymap.ActionNavPageDown:
		a.uniqueCtrl.MoveSelection(a.uniqueVisibleRows())
	case keymap.ActionNavTop:
		a.uniqueCtrl.SelectEdge(false)
	case keymap.ActionNavBottom:
		a.uniqueCtrl.SelectEdge(true)
	case keymap.ActionNavOpen:
		if path, ok := a.uniqueCtrl.OpenableFile(); ok {
			a.openFileExternally(path)
		} else {
			a.uniqueCtrl.ExpandOrDescend()
		}
	default:
		return false
	}
	a.uniqueCtrl.EnsureSelectionVisible(a.uniqueVisibleRows())
	return true
}

func (a *App) handleUniqueViewKey(event *tcell.EventKey) bool {
	// An open type-to-jump filter sees the key before the vi remap so h/j/k/l type into it.
	if f := a.uniqueCtrl.Filter(); !a.model.UniqueView.FocusPanel && f.UIActive() {
		if !filterRetainsKey(f, event, a.keys.Global) {
			a.uniqueCtrl.CancelFilter()
		} else if a.handleListFilterKey(a.uniqueCtrl, event) {
			a.uniqueCtrl.EnsureSelectionVisible(a.uniqueVisibleRows())
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
	if a.model.UniqueView.FocusPanel {
		a.handleUniqueDestKey(action, event)
		return false
	}
	if action == "" && keymap.IsPlainPrintableRune(event) {
		a.handleListFilterKey(a.uniqueCtrl, event)
		a.uniqueCtrl.EnsureSelectionVisible(a.uniqueVisibleRows())
		return false
	}
	if action != "" {
		a.tryDispatchUnique(action)
	}
	return false
}

// handleUniqueDestKey handles keys while the destination panel has focus: it is browsed like the
// duplicates view's panel, and copy and move still act on the tree's files. A delete needs the
// tree in focus, so it never acts on files out of view of the cursor by surprise.
func (a *App) handleUniqueDestKey(action string, event *tcell.EventKey) {
	id := a.inactivePanelID()
	p, rows := a.panelByID(id), a.panelViewportRows(id)
	if action == "" {
		action = auxPanelRawKeyAction(event)
	}
	switch action {
	case keymap.ActionUniqueClose, keymap.ActionFileIndexFindUnique, auxPanelRawKeyClose:
		a.closeUniqueView()
	case keymap.ActionPanelSwitch, keymap.ActionUniquePrevPane:
		a.uniqueCtrl.SwitchPane()
	case keymap.ActionCopy:
		a.dialogCtrl.ActivateCopyAction()
	case keymap.ActionMove:
		a.dialogCtrl.ActivateMoveAction()
	case keymap.ActionUniqueToggleNode:
		a.handleAuxPanelNavKey(p, rows, keymap.ActionNavOpen)
	case keymap.ActionUniqueCollapse:
		a.handleAuxPanelNavKey(p, rows, keymap.ActionNavParent)
	default:
		a.handleAuxPanelNavKey(p, rows, action)
	}
}

// activateUniqueHelpAction runs a row of the view's Help on the view's own dispatch.
func (a *App) activateUniqueHelpAction(actionID string) bool {
	switch actionID {
	case keymap.ActionAppQuit:
		return a.handleQuit()
	case keymap.ActionAppQuitImmediate:
		return a.handleQuitImmediate()
	case keymap.ActionAppOpenMenu:
		a.openMenu()
		return false
	}
	if !a.tryDispatchUnique(actionID) {
		a.tryDispatchAuxiliaryScreens(actionID)
	}
	return false
}

func uniqueViewFooterKeys(global, unique *keymap.Map) []menu.FunctionKey {
	var out []menu.FunctionKey
	add := func(km *keymap.Map, key tcell.Key, action, hint string) {
		if km == nil {
			return
		}
		if lbl := km.MenuBindingLabel(action); lbl != "" {
			out = append(out, menu.FunctionKey{Key: key, KeyLabel: lbl, Hint: hint, ActionID: action})
		}
	}
	add(global, tcell.KeyNUL, keymap.ActionPanelRefresh, "Refresh")
	add(global, tcell.KeyNUL, keymap.ActionFileView, "View")
	add(global, tcell.KeyNUL, keymap.ActionFileEdit, "Edit")
	add(global, tcell.KeyNUL, keymap.ActionCopy, "Copy")
	add(global, tcell.KeyNUL, keymap.ActionMove, "Move")
	add(global, tcell.KeyF8, keymap.ActionFileDelete, "Delete")
	add(unique, tcell.KeyNUL, keymap.ActionUniqueHashUnverified, "Hash")
	add(global, tcell.KeyNUL, keymap.ActionPanelClearSelection, "Unselect all")
	return out
}

// handleUniqueMouse handles the view's tree: a left click focuses it and selects the row, the wheel
// moves the cursor. Off the tree, it handles the destination panel.
func (a *App) handleUniqueMouse(layout ui.Layout, x, y int, buttons, pressed tcell.ButtonMask) (rendered bool) {
	row, ok := ui.UniqueListEntryAt(layout, &a.model, x, y)
	if !ok {
		return a.handleUniqueDestMouse(layout, x, y, buttons, pressed)
	}
	switch step := wheelDelta(buttons, config.MouseWheelRows); {
	case step != 0:
		a.uniqueCtrl.MoveSelection(step)
	case pressed&tcell.Button1 != 0:
		a.uniqueCtrl.SelectRow(row)
	default:
		return false
	}
	a.uniqueCtrl.EnsureSelectionVisible(a.uniqueVisibleRows())
	a.render()
	return true
}

// handleUniqueDestMouse handles the destination panel: a left click focuses it and moves its cursor
// to the entry under the pointer, the wheel moves the cursor.
func (a *App) handleUniqueDestMouse(layout ui.Layout, x, y int, buttons, pressed tcell.ButtonMask) (rendered bool) {
	row, ok := ui.UniqueDestEntryAt(layout, &a.model, x, y)
	if !ok {
		return false
	}
	id := a.inactivePanelID()
	p, rows := a.panelByID(id), a.panelViewportRows(id)
	switch step := wheelDelta(buttons, config.MouseWheelRows); {
	case step != 0:
		a.doListNav(func() { p.Move(step, rows) })
	case pressed&tcell.Button1 != 0:
		a.uniqueCtrl.FocusPanel()
		a.doListNav(func() { p.Move(row-p.Cursor, rows) })
	default:
		return false
	}
	a.render()
	return true
}
