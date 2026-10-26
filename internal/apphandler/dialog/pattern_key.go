package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// PatternKeys binds one of the twin pattern dialogs (Filter, Group Select; see
// dialog.PatternDialogView) to HandlePatternDialogKey.
type PatternKeys struct {
	Spec           dlg.Spec
	Focus          *dlg.ID
	Text           *string
	Cursor, Scroll *int
	QueryWidth     func() int
	Keys           *keymap.Map // ui.input.dialog bindings for the pattern editor
	OnEdit         func()      // after the pattern text changed; may be nil
	Confirm        func()      // OK, Enter in the pattern input or on a checkbox
	Cancel         func()
	Apply          func(dlg.Event) // a radio or checkbox activation
}

// HandlePatternDialogKey routes a key for an open pattern dialog. The engine owns focus and the
// mnemonics; the pattern input's text editing runs when the engine declines a key. Alt+letter
// keys that are mnemonics of either twin never edit, whether or not this dialog has them.
func HandlePatternDialogKey(p PatternKeys, event *tcell.EventKey) {
	if ev, ok := dlg.HandleKey(p.Spec, p.Focus, event); ok {
		ApplyPatternDialogEvent(p, ev)
		return
	}
	if *p.Focus != dialog.PatternInput {
		return
	}
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) && dialog.PatternAltIsMnemonic(event.Rune()) {
		return
	}
	scrollquery.HandleKey(p.Keys, event, true, scrollquery.NewEdit(p.Text, p.Cursor, p.Scroll, p.QueryWidth(), p.OnEdit))
}

// ApplyPatternDialogEvent acts on a widget event of a pattern dialog (from a key or a click).
func ApplyPatternDialogEvent(p PatternKeys, ev dlg.Event) {
	switch {
	case ev.Kind == dlg.EventCaret:
		*p.Cursor = ev.Index
	case ev.ID == dlg.OK || ev.ID == dialog.PatternInput:
		p.Confirm()
	case ev.ID == dlg.Cancel:
		p.Cancel()
	case ev.ID == "":
	case ev.Enter && !dialog.PatternIsRadio(ev.ID):
		p.Confirm() // Enter on a checkbox confirms; Space or a click toggles it
	default:
		p.Apply(ev)
	}
}
