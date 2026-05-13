package dialog

import (
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
