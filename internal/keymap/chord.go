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

// NormalizeAltMeta maps ModMeta into ModAlt. Many terminals encode the physical
// Alt/Option key as ModMeta (or Meta+Alt together) while our TOML "M-" prefix
// parses as ModAlt only, so lookup would otherwise miss those chords.
func NormalizeAltMeta(m tcell.ModMask) tcell.ModMask {
	if m&tcell.ModMeta != 0 {
		m = (m &^ tcell.ModMeta) | tcell.ModAlt
	}
	return m
}

// AltLetterModifiers reports whether m is Alt-only for a letter mnemonic after
// normalizing Meta→Alt (no Ctrl/Shift).
func AltLetterModifiers(m tcell.ModMask) bool {
	return NormalizeAltMeta(m) == tcell.ModAlt
}

// EventChord returns a normalized chord representation of ev for reverse-map keys.
func EventChord(ev *tcell.EventKey) Chord {
	if ev == nil {
		return Chord{}
	}
	mod := NormalizeAltMeta(ev.Modifiers())
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

	if key == tcell.KeyCtrlSpace {
		mod &^= tcell.ModCtrl
		return Chord{Key: tcell.KeyCtrlSpace, Rune: 0, Mod: mod}
	}
	if key >= tcell.KeyCtrlA && key <= tcell.KeyCtrlZ {
		mod &^= tcell.ModCtrl
		return Chord{Key: key, Rune: ch.Rune, Mod: mod}
	}
	// Terminals often deliver Ctrl+Space as NUL (fish_key_reader: bind -k nul).
	if key == tcell.KeyNUL || (key == tcell.KeyRune && ch.Rune == 0) {
		mod &^= tcell.ModCtrl
		return Chord{Key: tcell.KeyCtrlSpace, Rune: 0, Mod: mod}
	}
	// Terminals often deliver ^Letter as legacy ASCII control codes (KeySOH..KeySUB) with no ModCtrl
	// (fish_key_reader \cQ → KeyDC1). ParseKey("C-letter") stores KeyCtrl* instead. Do not remap
	// KeyBackspace/KeyTab/KeyEnter — tcell uses those same byte values for navigation keys.
	if key >= tcell.KeySOH && key <= tcell.KeySUB && key != tcell.KeyRune {
		switch key {
		case tcell.KeyBackspace, tcell.KeyTab, tcell.KeyEnter:
			return ch
		}
		mod &^= tcell.ModCtrl
		return Chord{Key: tcell.KeyCtrlA + key - tcell.KeySOH, Rune: 0, Mod: mod}
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
	if r == ' ' {
		return Chord{
			Key:  tcell.KeyCtrlSpace,
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
