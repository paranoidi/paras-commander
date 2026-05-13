package ui

import "github.com/gdamore/tcell/v2"

// AltDialogOK reports Alt+O (standard dialog OK mnemonic).
func AltDialogOK(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyRune && ev.Modifiers() == tcell.ModAlt &&
		(ev.Rune() == 'o' || ev.Rune() == 'O')
}

// AltDialogCancel reports Alt+C (standard dialog Cancel mnemonic).
func AltDialogCancel(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyRune && ev.Modifiers() == tcell.ModAlt &&
		(ev.Rune() == 'c' || ev.Rune() == 'C')
}
