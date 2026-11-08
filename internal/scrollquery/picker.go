package scrollquery

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// PickerBinding ties a fuzzy picker screen (run-for-each command history, mass rename patterns,
// column profiles) to its state: the dialog focus, the filter text, the list cursor and offset,
// and what the picker does.
type PickerBinding struct {
	Focus            *dlg.ID
	Query            *string
	Cursor, Scroll   *int
	Selected, Offset *int
	Count            func() int // ranked rows
	Resync           func()     // re-filter and re-rank after the filter text changed
	Activate         func()     // OK / Enter
	ClosePicker      func()     // Cancel
}

// PickerGeometry returns the visible list rows and the filter input's width of a picker screen,
// from its placed spec on a w by h screen.
func PickerGeometry(w, h int, spec dlg.Spec) (listRows, queryWidth int) {
	p, ok := dlg.Place(w, h, spec)
	listRows, queryWidth = 1, 10
	if it, found := p.Item(dialog.PickerList); ok && found {
		listRows = max(it.Rect.Height, 1)
	}
	if ok {
		queryWidth = max(dlg.InputTextWidth(w, h, spec, dialog.PickerFilter), 10)
	}
	return listRows, queryWidth
}

// HandlePickerKey routes a key for an open picker screen: the widget engine owns focus, list
// motion and the buttons; what it declines is editing of the filter text, which re-ranks the list
// and puts the cursor on its first row.
func HandlePickerKey(w, h int, spec dlg.Spec, inputKeys *keymap.Map, event *tcell.EventKey, b PickerBinding) {
	if ev, ok := dlg.HandleKey(spec, b.Focus, event); ok {
		ApplyPickerEvent(ev, b)
		return
	}
	if *b.Focus != "" && *b.Focus != dialog.PickerFilter {
		return
	}
	rows, width := PickerGeometry(w, h, spec)
	onChange := func() {
		b.Resync()
		*b.Selected = 0
		dialog.EnsurePickerScroll(b.Count(), b.Selected, b.Offset, rows)
	}
	edit := NewEdit(b.Query, b.Cursor, b.Scroll, width, onChange)
	HandleKey(inputKeys, event, true, edit)
}

// ApplyPickerEvent acts on a widget event of a picker screen (key or click).
func ApplyPickerEvent(ev dlg.Event, b PickerBinding) {
	switch {
	case ev.Kind == dlg.EventCaret:
		*b.Cursor = ev.Index
	case ev.Kind == dlg.EventListMove:
		*b.Selected, *b.Offset = ev.Index, ev.Offset
	case ev.ID == dlg.Cancel:
		b.ClosePicker()
	case ev.ID == dlg.OK || ev.ID == dialog.PickerFilter:
		b.Activate()
	}
}
