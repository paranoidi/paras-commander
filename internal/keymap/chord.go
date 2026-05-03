package keymap

import (
	"github.com/gdamore/tcell/v2"
	"unicode"
)

// Chord is a single key chord (no multi-stroke sequences).
type Chord struct {
	Key  tcell.Key
	Rune rune
	Mod  tcell.ModMask
}

// EventChord returns a normalized chord representation of ev for reverse-map keys.
func EventChord(ev *tcell.EventKey) Chord {
	if ev == nil {
		return Chord{}
	}
	mod := ev.Modifiers()
	key := ev.Key()
	if key == tcell.KeyBackspace2 {
		key = tcell.KeyBackspace
	}
	if key == tcell.KeyRune {
		return Chord{Key: tcell.KeyRune, Rune: ev.Rune(), Mod: mod}
	}
	return Chord{Key: key, Mod: mod}
}

// CanonicalChord collapses redundant modifier bits so parsed bindings match live terminal events.
//
// Chords parsed from "C-letter" store KeyCtrl* with Ctrl cleared from Mod; terminals often emit
// KeyCtrl* with ModCtrl still set. ^Letter may arrive as KeyRune 1–26 ("^D") or as rune 'd' plus ModCtrl.
func CanonicalChord(ch Chord) Chord {
	key := ch.Key
	mod := ch.Mod

	if key >= tcell.KeyCtrlA && key <= tcell.KeyCtrlZ {
		mod &^= tcell.ModCtrl
		return Chord{Key: key, Rune: ch.Rune, Mod: mod}
	}
	// tcell.NewEventKey(KeyRune, ^letter, Ctrl) collapses Key to legacy codes 1..26 (^A..^Z), whereas
	// ParseKey("C-letter") yields KeyCtrl* (Ctrl bit dropped). Only remap when ModCtrl indicates a control chord.
	if key >= 1 && key <= 26 && key != tcell.KeyRune && mod&tcell.ModCtrl != 0 {
		mod &^= tcell.ModCtrl
		return Chord{Key: tcell.KeyCtrlA + key - 1, Mod: mod}
	}
	if key != tcell.KeyRune || mod&tcell.ModCtrl == 0 {
		return ch
	}
	r := ch.Rune
	const ctrlLow = '\x01'
	const ctrlHigh = '\x1a'
	if r >= ctrlLow && r <= ctrlHigh {
		return Chord{
			Key:  tcell.KeyCtrlA + tcell.Key(r-ctrlLow),
			Rune: 0,
			Mod:  mod &^ tcell.ModCtrl,
		}
	}
	lr := unicode.ToLower(r)
	if lr >= 'a' && lr <= 'z' {
		return Chord{
			Key:  tcell.KeyCtrlA + tcell.Key(lr-'a'),
			Rune: 0,
			Mod:  mod &^ tcell.ModCtrl,
		}
	}
	return ch
}
