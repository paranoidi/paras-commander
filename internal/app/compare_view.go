package app

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	comparectrl "github.com/paranoidi/paras-commander/internal/apphandler/compare"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func (a *App) openComparePanels() {
	a.compareCtrl.Open()
}

func (a *App) closeCompareView() {
	a.compareCtrl.Close()
}

func (a *App) pollCompareUpdates(payload comparectrl.WakePayload) bool {
	return a.compareCtrl.PollUpdates(payload)
}

func (a *App) tryDispatchCompare(actionID string) bool {
	switch actionID {
	case keymap.ActionPanelComparePanels:
		if a.model.ViewMode == ui.ViewCompare {
			a.closeCompareView()
		} else {
			a.openComparePanels()
		}
		return true
	case keymap.ActionCompareClose:
		if a.model.ViewMode == ui.ViewCompare {
			a.closeCompareView()
		}
		return true
	case keymap.ActionCompareCycleFilter:
		if a.model.ViewMode == ui.ViewCompare {
			a.compareCtrl.OpenFilterDialog()
		}
		return true
	case keymap.ActionCompareResetFilter:
		if a.model.ViewMode == ui.ViewCompare {
			a.compareCtrl.SetFilter(comparepkg.FilterAll)
			a.model.CompareView.NameFilter, a.model.CompareView.NameFilterLabel = nil, ""
		}
		return true
	case keymap.ActionCompareFilterDialog:
		if a.model.ViewMode == ui.ViewCompare {
			a.dialogCtrl.OpenFilterDialog()
		}
		return true
	case keymap.ActionCompareRefresh:
		if a.model.ViewMode == ui.ViewCompare {
			a.compareCtrl.Refresh()
		}
		return true
	case keymap.ActionCompareMerge:
		if a.model.ViewMode == ui.ViewCompare {
			a.compareCtrl.OpenMergeDialog()
		}
		return true
	case keymap.ActionPanelSwitch:
		if a.model.ViewMode != ui.ViewCompare {
			return false
		}
		a.compareCtrl.ToggleColumnFocus()
		return true
	case keymap.ActionPanelSelectToggle:
		if a.model.ViewMode != ui.ViewCompare {
			return false
		}
		if conflicts := a.compareCtrl.ToggleColumnSelection(); conflicts {
			a.setTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
		}
		a.moveCompareSelection(1)
		return true
	case keymap.ActionPanelPinToggle:
		if a.model.ViewMode != ui.ViewCompare {
			return false
		}
		a.pinCtrl.ToggleCompareSelection()
		return true
	case keymap.ActionPanelSelectGroup:
		if a.model.ViewMode != ui.ViewCompare {
			return false
		}
		a.compareCtrl.OpenSelectDialog(true)
		return true
	case keymap.ActionPanelUnselectGroup:
		if a.model.ViewMode != ui.ViewCompare {
			return false
		}
		a.compareCtrl.OpenSelectDialog(false)
		return true
	case keymap.ActionPanelInvertSelection:
		if a.model.ViewMode != ui.ViewCompare {
			return false
		}
		a.compareCtrl.InvertColumnSelection()
		return true
	case keymap.ActionFileDelete:
		if a.model.ViewMode != ui.ViewCompare {
			return false
		}
		a.openCompareDeleteDialog()
		return true
	case keymap.ActionPanelExternalBrowser:
		if a.model.ViewMode != ui.ViewCompare {
			return false
		}
		a.compareOpenInExternalBrowser()
		return true
	case keymap.ActionCompareToggleEmpty:
		if a.model.ViewMode == ui.ViewCompare {
			a.compareCtrl.ToggleIgnoreEmpty()
		}
		return true
	default:
		return false
	}
}

// compareOpenInExternalBrowser reveals the focused column's file directory, or that side's
// panel directory when the row has no entry there.
func (a *App) compareOpenInExternalBrowser() {
	if path, _, ok := a.compareCtrl.SelectedColumnTarget(); ok {
		a.openDirInExternalBrowser(filepath.Dir(path))
		return
	}
	panelID := ui.PrimaryPanel
	if a.model.CompareView.FocusColumn == ui.CompareColumnSecondary {
		panelID = ui.SecondaryPanel
	}
	a.openPanelPathInExternalBrowser(panelID)
}

// openCompareDeleteDialog deletes the focused column's panel selection (set with Space) or,
// without one, the file under the cursor on that side.
func (a *App) openCompareDeleteDialog() {
	p := a.panelByID(ui.PrimaryPanel)
	if a.model.CompareView.FocusColumn == ui.CompareColumnSecondary {
		p = a.panelByID(ui.SecondaryPanel)
	}
	if len(p.SelectedPaths) > 0 {
		a.dialogCtrl.OpenDeleteDialog(p)
		return
	}
	path, _, ok := a.compareCtrl.SelectedColumnTarget()
	if !ok {
		a.setTransientMessage("Nothing to delete on this side", ui.MessageUrgencyInfo)
		return
	}
	a.dialogCtrl.OpenDeleteDialogForPath(path, filepath.Base(path))
}

// compareVisibleRows returns the merged twin-panel list row count used for Compare-list
// paging and scroll-into-view math (must match drawCompareView's PanelListRows).
func (a *App) compareVisibleRows() int {
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	rect := ui.MergeTwinPanelRects(layout.Primary, layout.Secondary, a.model.SplitOrientation)
	return ui.PanelListRows(rect)
}

// moveCompareSelection moves the Compare-list cursor by delta rows (clamped), scrolling into
// view. Shared by the raw arrow-key handling below and by help-dialog activation.
func (a *App) moveCompareSelection(delta int) {
	rows := a.compareCtrl.FilteredRows()
	st := &a.model.CompareView
	sel := max(0, st.Selected+delta)
	if len(rows) > 0 && sel > len(rows)-1 {
		sel = len(rows) - 1
	}
	st.Selected = sel
	a.compareCtrl.EnsureSelectionVisible(a.compareVisibleRows())
}

// selectCompareEdge moves the Compare-list cursor to the first (toEnd=false) or last
// (toEnd=true) row.
func (a *App) selectCompareEdge(toEnd bool) {
	rows := a.compareCtrl.FilteredRows()
	st := &a.model.CompareView
	if toEnd {
		if len(rows) > 0 {
			st.Selected = len(rows) - 1
		}
	} else {
		st.Selected = 0
	}
	a.compareCtrl.EnsureSelectionVisible(a.compareVisibleRows())
}

func (a *App) handleCompareViewKey(event *tcell.EventKey) bool {
	if a.model.CompareFilterDialog.Open {
		a.compareCtrl.HandleFilterDialogKey(event)
		return false
	}
	if a.model.CompareMergeDialog.Open {
		a.compareCtrl.HandleMergeDialogKey(event)
		return false
	}
	if a.model.ViMotionMode {
		event = keymap.RemapViMotionKey(event)
	}
	switch event.Key() {
	case tcell.KeyEsc:
		a.closeCompareView()
		return false
	case tcell.KeyLeft:
		a.compareCtrl.MoveColumnFocus(-1)
		return false
	case tcell.KeyRight:
		a.compareCtrl.MoveColumnFocus(1)
		return false
	}

	nextAction := a.actionFromKeyEvent(event)
	if result, handled := a.dispatchAuxiliaryViewCommonKeys(event, nextAction); handled {
		return result
	}
	if nextAction != "" && a.tryDispatchCompare(nextAction) {
		return false
	}
	if nextAction != "" && a.tryDispatchAuxiliaryScreens(nextAction) {
		return false
	}
	if nextAction == keymap.ActionPanelExternalBrowser {
		a.dispatch(nextAction)
		return false
	}

	visible := a.compareVisibleRows()
	switch event.Key() {
	case tcell.KeyUp:
		a.moveCompareSelection(-1)
	case tcell.KeyDown:
		a.moveCompareSelection(1)
	case tcell.KeyPgUp:
		a.moveCompareSelection(-visible)
	case tcell.KeyPgDn:
		a.moveCompareSelection(visible)
	case tcell.KeyHome:
		a.selectCompareEdge(false)
	case tcell.KeyEnd:
		a.selectCompareEdge(true)
	case tcell.KeyEnter:
		a.compareCtrl.DiscardReturn()
		a.compareCtrl.NavigateFromSelection(visible)
		a.closeCompareView()
	}
	return false
}

func compareViewFooterKeys(global, keys *keymap.Map, filter comparepkg.Filter, ignoreEmpty bool) []menu.FunctionKey {
	if keys == nil {
		return nil
	}
	var out []menu.FunctionKey
	if lbl := keys.MenuBindingLabel(keymap.ActionCompareCycleFilter); lbl != "" {
		filterLabel := comparepkg.FilterLabel(filter)
		out = append(out, menu.FunctionKey{
			KeyLabel:        lbl,
			Hint:            "Category",
			HintShiftPrefix: filterLabel,
			ActionID:        keymap.ActionCompareCycleFilter,
		})
	}
	if lbl := keys.MenuBindingLabel(keymap.ActionCompareToggleEmpty); lbl != "" {
		hint := "Ignore empty"
		if ignoreEmpty {
			hint = "Show empty"
		}
		out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: hint, ActionID: keymap.ActionCompareToggleEmpty})
	}
	out = append(out, menu.FunctionKey{Key: tcell.KeyF5, KeyLabel: "F5", Hint: "Merge", ActionID: keymap.ActionCompareMerge})
	if global != nil {
		if lbl := global.MenuBindingLabel(keymap.ActionFileDelete); lbl != "" {
			out = append(out, menu.FunctionKey{Key: tcell.KeyF8, KeyLabel: lbl, Hint: "Delete", ActionID: keymap.ActionFileDelete})
		}
	}
	return out
}
