package app

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func (a *App) togglePanelSelectionStash() {
	p := a.activePanel()
	if p.StashEmpty() {
		if !p.StashSaveFromSelection() {
			a.setTransientMessage("No selection to stash", ui.MessageUrgencyInfo)
			return
		}
		n := p.StashPathCount()
		a.setTransientMessage(fmt.Sprintf("%d selection(s) stashed", n), ui.MessageUrgencyInfo)
		a.render()
		return
	}
	if p.SelectedPathCount() == 0 {
		a.restorePanelStash(p)
		return
	}
	a.model.StashRestoreDialog = dialog.StashRestoreDialogState{Open: true, Focus: dialog.StashReplace}
	a.render()
}

func (a *App) restorePanelStash(p *panel.State) {
	paths := append([]string(nil), p.SelectionStashPaths...)
	strip := append([]string(nil), p.SelectionStashStripOrder...)
	p.ApplySelectionSnapshot(paths, strip, p.SelectionStashStripCursorPath)
	p.StashClear()
	n := p.SelectedPathCount()
	a.setTransientMessage(fmt.Sprintf("%d selection(s) restored from stash", n), ui.MessageUrgencyInfo)
	a.render()
}

func (a *App) closeStashRestoreDialog() {
	a.model.StashRestoreDialog = dialog.StashRestoreDialogState{}
}

func (a *App) handleStashRestoreDialogKey(event *tcell.EventKey) {
	d := &a.model.StashRestoreDialog
	if ev, ok := dlg.HandleKey(dialog.StashRestoreView(*d), &d.Focus, event); ok && ev.ID != "" {
		a.applyStashRestoreChoice(stashRestoreFocusAction(ev.ID))
	}
}

// stashRestoreFocusAction maps a stash restore button to its action; an unset focus is the first
// button (Replace).
func stashRestoreFocusAction(focus dlg.ID) string {
	switch focus {
	case dialog.StashMerge:
		return "merge"
	case dialog.StashDropStash:
		return "drop_stash"
	case dialog.StashDropAll:
		return "drop_all"
	case dlg.Cancel:
		return "cancel" // closes the dialog, keeps stash and selection
	}
	return "replace"
}

func (a *App) applyStashRestoreChoice(action string) {
	p := a.activePanel()
	stashPaths := append([]string(nil), p.SelectionStashPaths...)
	stashStrip := append([]string(nil), p.SelectionStashStripOrder...)
	stashCursor := p.SelectionStashStripCursorPath
	a.closeStashRestoreDialog()

	switch action {
	case "replace":
		p.ApplySelectionSnapshot(stashPaths, stashStrip, stashCursor)
		p.StashClear()
		a.setTransientMessage(fmt.Sprintf("%d selection(s) restored (replaced)", p.SelectedPathCount()), ui.MessageUrgencyInfo)
	case "merge":
		before := p.SelectedPathCount()
		p.MergeSelectionSnapshot(stashPaths, stashStrip, stashCursor)
		p.StashClear()
		added := p.SelectedPathCount() - before
		a.setTransientMessage(fmt.Sprintf("Merged %d stashed selection(s)", added), ui.MessageUrgencyInfo)
	case "drop_stash":
		p.StashClear()
		a.setTransientMessage("Stash dropped", ui.MessageUrgencyInfo)
	case "drop_all":
		p.ClearSelection()
		p.StashClear()
		a.setTransientMessage("Selection and stash cleared", ui.MessageUrgencyInfo)
	}
	a.render()
}
