package subshell

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// EncodeKey translates a tcell key event into the byte sequence a real
// terminal would send to a PTY for that key (xterm-compatible legacy
// encoding; no kitty keyboard protocol). appCursor selects DECCKM
// application-cursor-keys mode for the arrow/Home/End keys. Returns nil for
// keys with no terminal encoding.
func EncodeKey(ev *tcell.EventKey, appCursor bool) []byte {
	mod := ev.Modifiers()
	alt := mod&tcell.ModAlt != 0

	switch ev.Key() {
	case tcell.KeyRune:
		b := []byte(string(ev.Rune()))
		if alt {
			return append([]byte{0x1b}, b...)
		}
		return b

	case tcell.KeyEnter:
		return withAlt(alt, '\r')
	case tcell.KeyTab:
		return withAlt(alt, '\t')
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return withAlt(alt, 0x7f)
	case tcell.KeyEsc:
		return withAlt(alt, 0x1b)

	case tcell.KeyUp:
		return cursorSeq('A', mod, appCursor)
	case tcell.KeyDown:
		return cursorSeq('B', mod, appCursor)
	case tcell.KeyRight:
		return cursorSeq('C', mod, appCursor)
	case tcell.KeyLeft:
		return cursorSeq('D', mod, appCursor)
	case tcell.KeyHome:
		return cursorSeq('H', mod, appCursor)
	case tcell.KeyEnd:
		return cursorSeq('F', mod, appCursor)

	case tcell.KeyInsert:
		return tildeSeq(2, mod)
	case tcell.KeyDelete:
		return tildeSeq(3, mod)
	case tcell.KeyPgUp:
		return tildeSeq(5, mod)
	case tcell.KeyPgDn:
		return tildeSeq(6, mod)

	case tcell.KeyF1:
		return ss3Seq('P', mod)
	case tcell.KeyF2:
		return ss3Seq('Q', mod)
	case tcell.KeyF3:
		return ss3Seq('R', mod)
	case tcell.KeyF4:
		return ss3Seq('S', mod)
	case tcell.KeyF5:
		return tildeSeq(15, mod)
	case tcell.KeyF6:
		return tildeSeq(17, mod)
	case tcell.KeyF7:
		return tildeSeq(18, mod)
	case tcell.KeyF8:
		return tildeSeq(19, mod)
	case tcell.KeyF9:
		return tildeSeq(20, mod)
	case tcell.KeyF10:
		return tildeSeq(21, mod)
	case tcell.KeyF11:
		return tildeSeq(23, mod)
	case tcell.KeyF12:
		return tildeSeq(24, mod)
	}

	// Ctrl+letter and other C0 control keys: tcell reports these as
	// KeyCtrlSpace..KeyCtrlUnderscore, offset by 64 from the raw byte.
	if k := ev.Key(); k >= tcell.KeyCtrlSpace && k <= tcell.KeyCtrlUnderscore {
		return withAlt(alt, byte(k-tcell.KeyCtrlSpace))
	}

	return nil
}

// withAlt returns b, prefixed with ESC when alt is set.
func withAlt(alt bool, b byte) []byte {
	if alt {
		return []byte{0x1b, b}
	}
	return []byte{b}
}

// navMods reports whether any of the modifiers that force the CSI
// "1;<m>" modified form (as opposed to the plain SS3/CSI form) are set.
func navMods(mod tcell.ModMask) bool {
	return mod&(tcell.ModShift|tcell.ModAlt|tcell.ModCtrl) != 0
}

// modParam computes the xterm modifier parameter: 1 + shift(1) + alt(2) + ctrl(4).
func modParam(mod tcell.ModMask) int {
	m := 1
	if mod&tcell.ModShift != 0 {
		m += 1
	}
	if mod&tcell.ModAlt != 0 {
		m += 2
	}
	if mod&tcell.ModCtrl != 0 {
		m += 4
	}
	return m
}

// csiModified renders the xterm "\x1b[1;<m><final>" modified key form.
func csiModified(final byte, mod tcell.ModMask) []byte {
	return []byte(fmt.Sprintf("\x1b[1;%d%c", modParam(mod), final))
}

// cursorSeq encodes an arrow/Home/End key: plain CSI or SS3 depending on
// appCursor, or the CSI modified form when any modifier is present
// (modifiers always win over appCursor).
func cursorSeq(final byte, mod tcell.ModMask, appCursor bool) []byte {
	if navMods(mod) {
		return csiModified(final, mod)
	}
	if appCursor {
		return []byte{0x1b, 'O', final}
	}
	return []byte{0x1b, '[', final}
}

// ss3Seq encodes F1-F4: plain SS3, or the CSI modified form when a modifier
// is present.
func ss3Seq(final byte, mod tcell.ModMask) []byte {
	if navMods(mod) {
		return csiModified(final, mod)
	}
	return []byte{0x1b, 'O', final}
}

// tildeSeq encodes Insert/Delete/PgUp/PgDn/F5-F12: "\x1b[<n>~", or
// "\x1b[<n>;<m>~" when a modifier is present.
func tildeSeq(n int, mod tcell.ModMask) []byte {
	if navMods(mod) {
		return []byte(fmt.Sprintf("\x1b[%d;%d~", n, modParam(mod)))
	}
	return []byte(fmt.Sprintf("\x1b[%d~", n))
}
