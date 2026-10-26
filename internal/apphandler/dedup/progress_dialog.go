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
	if ev, ok := dlg.HandleKey(spec, &st.Focus, event); ok {
		h.ApplyProgressDialogEvent(ev)
	}
}

// ApplyProgressDialogEvent acts on a widget event of the scan-progress dialog (key or click).
func (h *Handler) ApplyProgressDialogEvent(ev dlg.Event) {
	switch ev.ID {
	case dlg.OK:
		h.Confirm()
	case dlg.Cancel:
		h.Close()
	}
}
