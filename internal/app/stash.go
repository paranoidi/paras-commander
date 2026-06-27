package app

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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
	a.model.StashRestoreDialog = dialog.StashRestoreDialogState{Open: true, Focus: 0}
	a.render()
}

func (a *App) restorePanelStash(p *panel.State) {
	paths := append([]string(nil), p.SelectionStashPaths...)
	strip := append([]string(nil), p.SelectionStashStripOrder...)
	p.ApplySelectionSnapshot(paths, strip)
	p.StashClear()
	n := p.SelectedPathCount()
	a.setTransientMessage(fmt.Sprintf("%d selection(s) restored from stash", n), ui.MessageUrgencyInfo)
	a.render()
}

func (a *App) closeStashRestoreDialog() {
	a.model.StashRestoreDialog = dialog.StashRestoreDialogState{}
}

func (a *App) handleStashRestoreDialogKey(event *tcell.EventKey) {
	if action, ok := stashRestoreDialogAltAction(event); ok {
		a.applyStashRestoreChoice(action)
		return
	}
	d := &a.model.StashRestoreDialog
	switch event.Key() {
	case tcell.KeyEsc:
		a.applyStashRestoreChoice("drop_stash")
	case tcell.KeyTab:
		d.Focus = (d.Focus + 1) % 4
	case tcell.KeyBacktab:
		d.Focus = (d.Focus + 3) % 4
	case tcell.KeyLeft:
		if d.Focus > 0 {
			d.Focus--
		}
	case tcell.KeyRight:
		if d.Focus < 3 {
			d.Focus++
		}
	case tcell.KeyEnter:
		a.applyStashRestoreChoice(stashRestoreFocusAction(d.Focus))
	}
}

func stashRestoreDialogAltAction(ev *tcell.EventKey) (string, bool) {
	if ev.Key() != tcell.KeyRune || !keymap.AltLetterModifiers(ev.Modifiers()) {
		return "", false
	}
	switch ev.Rune() {
	case 'r', 'R':
		return "replace", true
	case 'm', 'M':
		return "merge", true
	case 'd', 'D':
		return "drop_stash", true
	case 'a', 'A':
		return "drop_all", true
	default:
		return "", false
	}
}

func stashRestoreFocusAction(focus int) string {
	switch focus {
	case 0:
		return "replace"
	case 1:
		return "merge"
	case 2:
		return "drop_stash"
	default:
		return "drop_all"
	}
}

func (a *App) applyStashRestoreChoice(action string) {
	p := a.activePanel()
	stashPaths := append([]string(nil), p.SelectionStashPaths...)
	stashStrip := append([]string(nil), p.SelectionStashStripOrder...)
	a.closeStashRestoreDialog()

	switch action {
	case "replace":
		p.ApplySelectionSnapshot(stashPaths, stashStrip)
		p.StashClear()
		a.setTransientMessage(fmt.Sprintf("%d selection(s) restored (replaced)", p.SelectedPathCount()), ui.MessageUrgencyInfo)
	case "merge":
		before := p.SelectedPathCount()
		p.MergeSelectionSnapshot(stashPaths, stashStrip)
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
