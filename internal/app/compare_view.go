package app

import (
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
	case keymap.ActionCompareToggleEmpty:
		if a.model.ViewMode == ui.ViewCompare {
			a.compareCtrl.ToggleIgnoreEmpty()
		}
		return true
	default:
		return false
	}
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

	if nextAction == keymap.ActionPanelSelectToggle {
		if conflicts := a.compareCtrl.ToggleColumnSelection(); conflicts {
			a.setTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
		}
		rows := a.compareCtrl.FilteredRows()
		st := &a.model.CompareView
		if st.Selected < len(rows)-1 {
			st.Selected++
		}
		a.compareCtrl.EnsureSelectionVisible(a.compareVisibleRows())
		return false
	}
	if nextAction == keymap.ActionPanelPinToggle {
		a.pinCtrl.ToggleCompareSelection()
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

func compareViewFooterKeys(keys *keymap.Map, filter comparepkg.Filter, ignoreEmpty bool) []menu.FunctionKey {
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
	return out
}
