package dedup

import (
	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
)

// HandleProgressDialogKey routes keys for the open scan-progress dialog (walking / hash-size
// confirmation / hashing phases).
func (h *Handler) HandleProgressDialogKey(event *tcell.EventKey) {
	st := &h.model.DedupProgressDialog
	snap := h.model.DedupSnapshot
	confirmGate := snap.Phase == comparepkg.DedupAwaitConfirm

	switch event.Key() {
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
			h.Confirm()
		} else {
			h.Close()
		}
	}
}
