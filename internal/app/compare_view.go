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
			a.openCompareFilterDialog()
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
			a.openCompareMergeDialog()
		}
		return true
	default:
		return false
	}
}

func (a *App) handleCompareViewKey(event *tcell.EventKey) bool {
	if a.model.CompareFilterDialog.Open {
		a.handleCompareFilterDialogKey(event)
		return false
	}
	if a.model.CompareMergeDialog.Open {
		a.handleCompareMergeDialogKey(event)
		return false
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
	if nextAction == keymap.ActionAppQuit {
		return a.handleQuit()
	}
	if nextAction == keymap.ActionAppQuitImmediate {
		return a.handleQuitImmediate()
	}
	if nextAction == keymap.ActionAppOpenMenu {
		a.openMenu()
		return false
	}
	if a.tryOpenMenuByShortcut(event) {
		return false
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
		width, height := a.screen.Size()
		layout := a.layoutForTerminalSize(width, height)
		rect := ui.MergeTwinPanelRects(layout.Primary, layout.Secondary, a.model.SplitOrientation)
		a.compareCtrl.EnsureSelectionVisible(max(0, rect.Height-2))
		return false
	}

	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	rect := ui.MergeTwinPanelRects(layout.Primary, layout.Secondary, a.model.SplitOrientation)
	visible := max(0, rect.Height-2)

	rows := a.compareCtrl.FilteredRows()
	st := &a.model.CompareView
	switch event.Key() {
	case tcell.KeyUp:
		if st.Selected > 0 {
			st.Selected--
		}
		a.compareCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyDown:
		if st.Selected < len(rows)-1 {
			st.Selected++
		}
		a.compareCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyPgUp:
		if visible > 0 {
			st.Selected -= visible
			if st.Selected < 0 {
				st.Selected = 0
			}
		}
		a.compareCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyPgDn:
		if visible > 0 && len(rows) > 0 {
			st.Selected += visible
			if st.Selected >= len(rows) {
				st.Selected = len(rows) - 1
			}
		}
		a.compareCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyHome:
		st.Selected = 0
		a.compareCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyEnd:
		if len(rows) > 0 {
			st.Selected = len(rows) - 1
		}
		a.compareCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyEnter:
		a.compareCtrl.NavigateFromSelection(visible)
		a.closeCompareView()
	}
	return false
}

func compareViewFooterKeys(keys *keymap.Map, filter comparepkg.Filter) []menu.FunctionKey {
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
		})
	}
	out = append(out, menu.FunctionKey{Key: tcell.KeyF5, KeyLabel: "F5", Hint: "Merge"})
	return out
}
