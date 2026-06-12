package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// linearFormHandlers configures handleLinearFormDialogKey.
type linearFormHandlers struct {
	focus              *int
	onApply            func()
	onCancel           func()
	onMnemonic         func(rune) bool
	onSpace            func(focus int) bool
	onMoveFocus        func(focus int, key tcell.Key) (int, bool)
	allowPlainOKCancel bool
}

func (a *App) handleLinearFormDialogKey(ev *tcell.EventKey, form ui.DialogLinearForm, h linearFormHandlers) bool {
	if a.tryStandardDialogActions(ev, h.onApply, h.onCancel, nil) {
		return true
	}
	switch ev.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		h.onCancel()
		return true
	case tcell.KeyEnter:
		if *h.focus == form.CancelIndex() {
			h.onCancel()
		} else {
			h.onApply()
		}
		return true
	case tcell.KeyRune:
		if keymap.AltLetterModifiers(ev.Modifiers()) {
			if h.onMnemonic != nil && h.onMnemonic(ev.Rune()) {
				return true
			}
			break
		}
		if ev.Modifiers() != tcell.ModNone {
			break
		}
		if h.onMnemonic != nil && h.onMnemonic(ev.Rune()) {
			return true
		}
		if h.allowPlainOKCancel {
			switch ev.Rune() {
			case 'o', 'O':
				h.onApply()
				return true
			case 'c', 'C':
				h.onCancel()
				return true
			}
		}
		if ev.Rune() == ' ' && h.onSpace != nil && h.onSpace(*h.focus) {
			return true
		}
	}
	if h.onMoveFocus != nil {
		if nf, ok := h.onMoveFocus(*h.focus, ev.Key()); ok {
			*h.focus = nf
			return true
		}
	}
	if nf, ok := form.MoveFocus(*h.focus, ev.Key()); ok {
		*h.focus = nf
		return true
	}
	return false
}
