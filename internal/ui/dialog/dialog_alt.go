package dialog

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
)

// AltLetter reports Alt+<letter> (either case) for a dialog-specific button mnemonic.
func AltLetter(ev *tcell.EventKey, letter rune) bool {
	return ev.Key() == tcell.KeyRune && keymap.AltLetterModifiers(ev.Modifiers()) &&
		unicode.ToLower(ev.Rune()) == letter
}

// ExtraMnemonic pairs an Alt+letter shortcut with an action, for dialogs that need
// additional Alt-letter mnemonics beyond OK/Cancel (e.g. Alt+Y/Alt+N on the delete
// confirmation, Alt+P "Add paused" on transfer).
type ExtraMnemonic struct {
	Rune rune
	Fn   func()
}

// TryDialogMnemonics handles the dialog-specific Alt-letter mnemonics in extras. OK and
// Cancel are not handled here: App.handleKey resolves ui.input.dialog-ok / dialog-cancel once
// for every dialog. Returns true when a matching action ran.
func TryDialogMnemonics(ev *tcell.EventKey, extras []ExtraMnemonic) bool {
	if ev.Key() != tcell.KeyRune || !keymap.AltLetterModifiers(ev.Modifiers()) {
		return false
	}
	for _, extra := range extras {
		if runeMatchesCaseFold(ev.Rune(), extra.Rune) {
			if extra.Fn != nil {
				extra.Fn()
			}
			return true
		}
	}
	return false
}

func runeMatchesCaseFold(got, want rune) bool {
	return got == want || unicode.ToLower(got) == unicode.ToLower(want)
}
