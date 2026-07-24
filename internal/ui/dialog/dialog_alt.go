package dialog

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
)

// AltDialogOK reports Alt+O (standard dialog OK mnemonic).
func AltDialogOK(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyRune && keymap.AltLetterModifiers(ev.Modifiers()) &&
		(ev.Rune() == 'o' || ev.Rune() == 'O')
}

// AltDialogCancel reports Alt+C (standard dialog Cancel mnemonic).
func AltDialogCancel(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyRune && keymap.AltLetterModifiers(ev.Modifiers()) &&
		(ev.Rune() == 'c' || ev.Rune() == 'C')
}

// ExtraMnemonic pairs an Alt+letter shortcut with an action, for dialogs that need
// standard Alt+O/Alt+C plus one or more additional Alt-letter mnemonics (e.g.
// Alt+Y/Alt+N on the delete confirmation, Alt+P "Add paused" on transfer).
type ExtraMnemonic struct {
	Rune rune
	Fn   func()
}

// TryStandardDialogActions handles Alt+O (apply) and Alt+C (cancel), plus any extra
// Alt-letter mnemonics. Returns true when a matching action ran.
func TryStandardDialogActions(ev *tcell.EventKey, apply, cancel func(), extras []ExtraMnemonic) bool {
	if ev.Key() != tcell.KeyRune || !keymap.AltLetterModifiers(ev.Modifiers()) {
		return false
	}
	if AltDialogOK(ev) {
		if apply != nil {
			apply()
		}
		return true
	}
	if AltDialogCancel(ev) {
		if cancel != nil {
			cancel()
		}
		return true
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
