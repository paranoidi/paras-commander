package dedup

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// HandleProgressDialogKey routes keys for the open scan-progress dialog (walking / hash-size
// confirmation / hashing phases): the engine owns focus and mnemonics, OK confirms the hash
// run and Cancel closes the view.
func (h *Handler) HandleProgressDialogKey(event *tcell.EventKey) {
	st := &h.model.DedupProgressDialog
	spec := dialog.DedupProgressView(h.model.DedupSnapshot, "", time.Time{})
	ev, ok := dlg.HandleKey(spec, &st.Focus, event)
	if !ok {
		return
	}
	switch ev.ID {
	case dlg.OK:
		h.Confirm()
	case dlg.Cancel:
		h.Close()
	}
}
