package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/dialogform"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func (a *App) handleLinearFormDialogKey(ev *tcell.EventKey, form dialog.DialogLinearForm, h dialogform.Handlers) bool {
	if a.tryStandardDialogActions(ev, h.OnApply, h.OnCancel, nil) {
		return true
	}
	return dialogform.HandleKey(ev, form, h)
}
