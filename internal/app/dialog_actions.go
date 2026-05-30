package app

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// dialogExtraMnemonic pairs an Alt+letter shortcut with an action.
type dialogExtraMnemonic struct {
	rune rune
	fn   func()
}

// tryStandardDialogActions handles Alt+O/C and optional extra Alt mnemonics.
// Returns true when a matching action ran.
func (a *App) tryStandardDialogActions(ev *tcell.EventKey, apply, cancel func(), extras []dialogExtraMnemonic) bool {
	if ev.Key() != tcell.KeyRune || !keymap.AltLetterModifiers(ev.Modifiers()) {
		return false
	}
	if ui.AltDialogOK(ev) {
		if apply != nil {
			apply()
		}
		return true
	}
	if ui.AltDialogCancel(ev) {
		if cancel != nil {
			cancel()
		}
		return true
	}
	for _, extra := range extras {
		if runeMatchesCaseFold(ev.Rune(), extra.rune) {
			if extra.fn != nil {
				extra.fn()
			}
			return true
		}
	}
	return false
}

func runeMatchesCaseFold(got, want rune) bool {
	return got == want || unicode.ToLower(got) == unicode.ToLower(want)
}
