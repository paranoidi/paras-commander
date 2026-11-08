package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// pickerGeometry returns the visible list rows and the filter input's width of the open file
// dialog's picker screen, from its placed spec.
func (h *Handler) pickerGeometry(spec dlg.Spec) (listRows, queryWidth int) {
	w, ht := h.screen.Size()
	return scrollquery.PickerGeometry(w, ht, spec)
}

// pickerListRows is the visible list rows of the open picker screen.
func (h *Handler) pickerListRows() int {
	rows, _ := h.pickerGeometry(h.FileDialogSpec())
	return rows
}

// handlePickerKey routes a key for the file dialog's open picker screen.
func (h *Handler) handlePickerKey(event *tcell.EventKey, b scrollquery.PickerBinding) {
	w, ht := h.screen.Size()
	scrollquery.HandlePickerKey(w, ht, h.FileDialogSpec(), h.keysDialogInput, event, b)
}

// openPickerBinding returns the binding of the file dialog's open picker screen, if any.
func (h *Handler) openPickerBinding() (scrollquery.PickerBinding, bool) {
	d := &h.model.FileDialog
	switch {
	case d.DialogType == dialog.FileDialogRunForEach && d.RunForEachHistoryOpen:
		return h.runForEachHistoryPickerBinding(), true
	case d.DialogType == dialog.FileDialogMassRename && massRenamePickerPhaseOpen(d.MassRenamePhase):
		return h.massRenamePickerBinding(), true
	}
	return scrollquery.PickerBinding{}, false
}
