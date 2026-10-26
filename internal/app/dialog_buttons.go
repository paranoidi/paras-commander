package app

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
)

// dispatchDialogButton runs the ui.input.dialog-ok / dialog-cancel chord in event against
// the top-most dialog. It is the only place those chords are handled: dialog key handlers
// never look for Alt+O / Alt+C / Esc / F9 themselves. Reports whether the key was consumed.
// Dialogs that expose no buttons (views, menus, quick filter) are left to their own handlers.
func (a *App) dispatchDialogButton(event *tcell.EventKey) bool {
	if a.keys.DialogInput == nil {
		return false
	}
	id, hit := a.keys.DialogInput.Lookup(event)
	if !hit && event.Key() == tcell.KeyRune && unicode.IsUpper(event.Rune()) {
		// Alt+Shift+O / Alt+Shift+C (caps lock) press OK / Cancel like their lowercase chord.
		id, hit = a.keys.DialogInput.Lookup(tcell.NewEventKey(tcell.KeyRune, unicode.ToLower(event.Rune()), event.Modifiers()))
	}
	if !hit || (id != keymap.ActionDialogOK && id != keymap.ActionDialogCancel) {
		return false
	}
	d, _ := a.activeDialog(event)
	b := d.buttons
	if b.none() {
		return false
	}
	fn := b.OK
	if id == keymap.ActionDialogCancel {
		fn = b.Cancel
		if event.Key() == tcell.KeyEsc && b.Esc != nil {
			fn = b.Esc
		}
	}
	if fn != nil {
		fn()
	}
	a.render()
	return true
}
