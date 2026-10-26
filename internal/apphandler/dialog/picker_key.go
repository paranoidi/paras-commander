package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// pickerBinding ties a fuzzy picker sub-screen (run-for-each command history, mass rename
// pattern pickers) to its state: the dialog focus, the filter text, the list cursor and offset,
// and what the picker does.
type pickerBinding struct {
	focus            *dlg.ID
	query            *string
	cursor, scroll   *int
	selected, offset *int
	count            func() int // ranked rows
	resync           func()     // re-filter and re-rank after the filter text changed
	activate         func()     // OK / Enter
	closePicker      func()     // Cancel
}

// pickerGeometry returns the visible list rows and the filter input's width of the open file
// dialog's picker screen, from its placed spec.
func (h *Handler) pickerGeometry(spec dlg.Spec) (listRows, queryWidth int) {
	w, ht := h.screen.Size()
	p, ok := dlg.Place(w, ht, spec)
	listRows, queryWidth = 1, 10
	if it, found := p.Item(dialog.PickerList); ok && found {
		listRows = max(it.Rect.Height, 1)
	}
	if it, found := p.Item(dialog.PickerFilter); ok && found {
		queryWidth = max(it.Rect.Width, 10)
	}
	return listRows, queryWidth
}

// pickerListRows is the visible list rows of the open picker screen.
func (h *Handler) pickerListRows() int {
	rows, _ := h.pickerGeometry(h.FileDialogSpec())
	return rows
}

// handlePickerKey routes a key for an open picker screen: the widget engine owns focus, list
// motion and the buttons; what it declines is editing of the filter text, which re-ranks the list
// and puts the cursor on its first row.
func (h *Handler) handlePickerKey(event *tcell.EventKey, b pickerBinding) {
	spec := h.FileDialogSpec()
	ev, ok := dlg.HandleKey(spec, b.focus, event)
	switch {
	case !ok:
		if *b.focus != "" && *b.focus != dialog.PickerFilter {
			return
		}
		rows, width := h.pickerGeometry(spec)
		onChange := func() {
			b.resync()
			*b.selected = 0
			dialog.EnsurePickerScroll(b.count(), b.selected, b.offset, rows)
		}
		edit := scrollquery.NewEdit(b.query, b.cursor, b.scroll, width, onChange)
		scrollquery.HandleKey(h.keysDialogInput, event, true, edit)
	case ev.Kind == dlg.EventListMove:
		*b.selected, *b.offset = ev.Index, ev.Offset
	case ev.ID == dlg.Cancel:
		b.closePicker()
	case ev.ID == dlg.OK || ev.ID == dialog.PickerFilter:
		b.activate()
	}
}
