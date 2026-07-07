package app

import (
	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func (a *App) handleDedupProgressDialogKey(event *tcell.EventKey) {
	st := &a.model.DedupProgressDialog
	snap := a.model.DedupSnapshot
	confirmGate := snap.Phase == comparepkg.DedupAwaitConfirm

	if dialog.AltDialogCancel(event) {
		a.closeDedupView()
		return
	}
	if confirmGate && dialog.AltDialogOK(event) {
		a.dedupCtrl.Confirm()
		return
	}

	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		a.closeDedupView()
		return
	case tcell.KeyLeft:
		if confirmGate && st.ButtonFocus > 0 {
			st.ButtonFocus--
		}
		return
	case tcell.KeyRight:
		if confirmGate && st.ButtonFocus < 1 {
			st.ButtonFocus++
		}
		return
	case tcell.KeyEnter:
		if confirmGate && st.ButtonFocus == 0 {
			a.dedupCtrl.Confirm()
		} else {
			a.closeDedupView()
		}
	}
}
