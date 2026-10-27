package app

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// resolveSelectionBeforeCompare runs open at once when neither main panel has a live selection.
// Otherwise it asks what to do with the selection first: Compare's selection, delete and merge
// act on the panel selections, so paths selected before Compare would be acted on unseen.
func (a *App) resolveSelectionBeforeCompare(open func()) {
	n := a.model.Primary.SelectedPathCount() + a.model.Secondary.SelectedPathCount()
	if n == 0 {
		open()
		return
	}
	a.compareSelectionOpen = open
	a.model.CompareSelection = dialog.CompareSelectionDialogState{Open: true, Focus: dialog.CompareSelectionStash, Selected: n}
}

func (a *App) handleCompareSelectionKey(event *tcell.EventKey) {
	d := &a.model.CompareSelection
	if ev, ok := dlg.HandleKey(dialog.CompareSelectionView(*d), &d.Focus, event); ok {
		a.applyCompareSelectionEvent(ev)
	}
}

// applyCompareSelectionEvent answers the compare selection dialog (key or click).
func (a *App) applyCompareSelectionEvent(ev dlg.Event) {
	if ev.ID != "" {
		a.answerCompareSelection(ev.ID)
	}
}

// answerCompareSelection drops or stashes both main panels' selection and opens Compare, or
// (Cancel) closes the dialog leaving the selection and the view as they were. Stash merges into
// a panel's existing stash.
func (a *App) answerCompareSelection(choice dlg.ID) {
	open := a.compareSelectionOpen
	n := a.model.CompareSelection.Selected
	a.compareSelectionOpen = nil
	a.model.CompareSelection = dialog.CompareSelectionDialogState{}
	switch choice {
	case dialog.CompareSelectionDrop:
		a.model.Primary.ClearSelection()
		a.model.Secondary.ClearSelection()
		a.setTransientMessage(fmt.Sprintf("%d selection(s) dropped", n), ui.MessageUrgencyInfo)
	case dialog.CompareSelectionStash:
		a.model.Primary.StashSaveFromSelection()
		a.model.Secondary.StashSaveFromSelection()
		a.setTransientMessage(fmt.Sprintf("%d selection(s) stashed", n), ui.MessageUrgencyInfo)
	default:
		return
	}
	if open != nil {
		open()
	}
}
