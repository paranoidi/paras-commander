package dedup

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

const dedupReturnButtons = 3 // Show, Rescan, Cancel

// HandleReturnDialogKey routes keys for the open kept-results dialog.
func (h *Handler) HandleReturnDialogKey(event *tcell.EventKey) {
	st := &h.model.DedupReturnDialog

	if dialog.AltDialogCancel(event) {
		h.returnDialogActivate(2)
		return
	}
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 's', 'S':
			h.returnDialogActivate(0)
		case 'r', 'R':
			h.returnDialogActivate(1)
		}
		return
	}

	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		h.returnDialogActivate(2)
	case tcell.KeyLeft:
		if st.ButtonFocus > 0 {
			st.ButtonFocus--
		}
	case tcell.KeyRight:
		if st.ButtonFocus < dedupReturnButtons-1 {
			st.ButtonFocus++
		}
	case tcell.KeyEnter:
		h.returnDialogActivate(st.ButtonFocus)
	}
}

// returnDialogActivate closes the dialog and runs button i (0=Show, 1=Rescan, 2=Cancel).
func (h *Handler) returnDialogActivate(i int) {
	h.model.DedupReturnDialog = dialog.DedupReturnDialogState{}
	switch i {
	case 0:
		h.VerifyAndShowKept()
	case 1:
		h.openRoot(h.activePanelPath())
	}
}
