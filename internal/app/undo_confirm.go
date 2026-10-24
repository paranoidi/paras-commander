package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// handleUndoConfirmKey drives the undo confirmation; Yes queues the undo job.
func (a *App) handleUndoConfirmKey(event *tcell.EventKey) bool {
	d := &a.model.UndoConfirm
	if ev, ok := dlg.HandleKey(dialog.UndoConfirmView(*d), &d.Focus, event); ok && ev.ID != "" {
		a.answerUndoConfirm(ev.ID == dialog.UndoYes)
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
