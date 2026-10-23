package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// handleUndoConfirmKey drives the undo confirmation; Yes queues the undo job.
func (a *App) handleUndoConfirmKey(event *tcell.EventKey) bool {
	d := &a.model.UndoConfirm
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 'y', 'Y':
			a.answerUndoConfirm(true)
		case 'n', 'N':
			a.answerUndoConfirm(false)
		}
		return false
	}
	switch event.Key() {
	case tcell.KeyLeft:
		d.Focus = dialog.DialogPairLeftRight(d.Focus, false)
	case tcell.KeyRight:
		d.Focus = dialog.DialogPairLeftRight(d.Focus, true)
	case tcell.KeyEnter:
		a.answerUndoConfirm(d.Focus == 0)
	}
	return false
}

// answerUndoConfirm closes the undo confirmation; yes queues the undo job.
func (a *App) answerUndoConfirm(yes bool) {
	id := a.model.UndoConfirm.JobID
	a.model.UndoConfirm = dialog.UndoConfirmState{}
	if yes {
		a.jobsCtrl.ConfirmUndo(id)
	}
}
