// Package dialogform implements the App-independent core of linear-form dialog
// key handling: focus navigation, mnemonics, space-toggle, and apply/cancel keys
// shared by every dialog built on dialog.DialogLinearForm (sort, listing-format,
// config, debounce-calibrate, group-select, run-for-each, and similar dialogs).
package dialogform

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// Handlers configures HandleKey.
type Handlers struct {
	Focus              *int
	OnApply            func()
	OnCancel           func()
	OnMnemonic         func(rune) bool
	OnSpace            func(focus int) bool
	OnMoveFocus        func(focus int, key tcell.Key) (int, bool)
	AllowPlainOKCancel bool
}

// HandleKey applies standard linear-form navigation/mnemonic/space-toggle/apply/cancel
// key handling. Callers must first try their dialog's Alt+O/Alt+C standard-actions
// handling (e.g. App.tryStandardDialogActions) before calling HandleKey, since that
// App-level wiring lives outside this App-independent package.
func HandleKey(ev *tcell.EventKey, form dialog.DialogLinearForm, h Handlers) bool {
	switch ev.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		h.OnCancel()
		return true
	case tcell.KeyEnter:
		if *h.Focus == form.CancelIndex() {
			h.OnCancel()
		} else if h.OnSpace != nil && h.OnSpace(*h.Focus) {
			// Enter activates the focused row (radio/checkbox) like Space; OK applies only
			// when OnSpace does not handle the current focus (e.g. plain input rows).
		} else {
			h.OnApply()
		}
		return true
	case tcell.KeyRune:
		if keymap.AltLetterModifiers(ev.Modifiers()) {
			if h.OnMnemonic != nil && h.OnMnemonic(ev.Rune()) {
				return true
			}
			break
		}
		if ev.Modifiers() != tcell.ModNone {
			break
		}
		if h.OnMnemonic != nil && h.OnMnemonic(ev.Rune()) {
			return true
		}
		if h.AllowPlainOKCancel {
			switch ev.Rune() {
			case 'o', 'O':
				h.OnApply()
				return true
			case 'c', 'C':
				h.OnCancel()
				return true
			}
		}
		if ev.Rune() == ' ' && h.OnSpace != nil && h.OnSpace(*h.Focus) {
			return true
		}
	}
	if h.OnMoveFocus != nil {
		if nf, ok := h.OnMoveFocus(*h.Focus, ev.Key()); ok {
			*h.Focus = nf
			return true
		}
	}
	if nf, ok := form.MoveFocus(*h.Focus, ev.Key()); ok {
		*h.Focus = nf
		return true
	}
	return false
}
