package subshell

import (
	"strconv"
	"strings"
)

// ToggleKeyCtrlO is the raw byte for Ctrl+O (MC subshell toggle) in non-kitty terminals.
const ToggleKeyCtrlO = 0x0f

// maxIncompleteCSIHold is how many trailing bytes to keep when a Ctrl+O CSI may be incomplete.
const maxIncompleteCSIHold = 32

// ContainsToggleKey reports whether b contains a subshell toggle keystroke.
func ContainsToggleKey(b []byte) bool {
	_, _, ok := FindToggle(b)
	return ok
}

// SplitOnToggleKey splits b at the first toggle. ok is false when no toggle is present.
func SplitOnToggleKey(b []byte) (before, after []byte, ok bool) {
	at, length, ok := FindToggle(b)
	if !ok {
		return nil, nil, false
	}
	return append([]byte(nil), b[:at]...), b[at+length:], true
}

// FindToggle locates the first MC-style subshell toggle in b (Ctrl+O byte or kitty/xterm CSI).
func FindToggle(b []byte) (at int, length int, ok bool) {
	for i := 0; i < len(b); i++ {
		if b[i] == ToggleKeyCtrlO {
			return i, 1, true
		}
		if n, isToggle := kittyCtrlOToggleLen(b[i:]); isToggle {
			return i, n, true
		}
		if n, isToggle := xtermCtrlOToggleLen(b[i:]); isToggle {
			return i, n, true
		}
	}
	return 0, 0, false
}

// SafePTYFlushLen returns how many leading bytes of b are safe to forward to the PTY without
// splitting a Ctrl+O prefix. Other kitty CSIs (arrows, letters) are flushed immediately.
func SafePTYFlushLen(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	if at, _, ok := FindToggle(b); ok {
		return at
	}
	if hold := incompleteCtrlOPrefixHold(b); hold >= 0 {
		if len(b)-hold > maxIncompleteCSIHold {
			return hold + 1
		}
		return hold
	}
	return len(b)
}

func incompleteCtrlOPrefixHold(b []byte) int {
	for i := 0; i < len(b); i++ {
		if b[i] != 0x1b {
			continue
		}
		rest := b[i:]
		if len(rest) == 1 {
			return i
		}
		if rest[1] != '[' {
			return -1
		}
		if _, isToggle := kittyCtrlOToggleLen(rest); isToggle {
			return -1
		}
		if _, isToggle := xtermCtrlOToggleLen(rest); isToggle {
			return -1
		}
		if couldBeIncompleteCtrlOCSI(rest) {
			return i
		}
	}
	return -1
}

func couldBeIncompleteCtrlOCSI(b []byte) bool {
	if len(b) < 2 || b[0] != 0x1b || b[1] != '[' {
		return false
	}
	body := string(b[2:])
	if body == "" {
		return true
	}
	for _, prefix := range []string{"1", "11", "15", "111"} {
		if strings.HasPrefix(prefix, body) || strings.HasPrefix(body, prefix) {
			if !csiTerminated(body) {
				return true
			}
		}
	}
	return false
}

func csiTerminated(body string) bool {
	return strings.ContainsAny(body, "u~")
}

// kittyCtrlOToggleLen reports whether b begins with a complete kitty keyboard-protocol Ctrl+O CSI.
func kittyCtrlOToggleLen(b []byte) (int, bool) {
	if len(b) < 4 || b[0] != 0x1b || b[1] != '[' {
		return 0, false
	}
	end := -1
	for i := 2; i < len(b); i++ {
		if b[i] == 'u' {
			end = i
			break
		}
		if b[i] < 0x20 && b[i] != '\t' {
			return 0, false
		}
	}
	if end < 0 {
		return 0, false
	}
	if !isKittyCtrlOCSI(b[2:end]) {
		return end + 1, false
	}
	return end + 1, true
}

// xtermCtrlOToggleLen matches legacy CSI terminated with ~ (e.g. foot/xterm ctrl+o).
func xtermCtrlOToggleLen(b []byte) (int, bool) {
	const seq = "\x1b[15;5~"
	if len(b) < len(seq) {
		return 0, false
	}
	if string(b[:len(seq)]) == seq {
		return len(seq), true
	}
	return 0, false
}

// isKittyCtrlOCSI parses the body of ESC [ … u (kitty / foot keyboard protocol).
func isKittyCtrlOCSI(body []byte) bool {
	parts := strings.Split(string(body), ";")
	if len(parts) < 2 {
		return false
	}
	cp, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	if cp != int(ToggleKeyCtrlO) && cp != int('o') {
		return false
	}
	const (
		modCtrl = 4
		modCaps = 0x40
		modNum  = 0x80
	)
	modField := parts[1]
	if i := strings.IndexByte(modField, ':'); i >= 0 {
		modField = modField[:i]
	}
	modEnc, err := strconv.Atoi(modField)
	if err != nil || modEnc == 0 {
		return false
	}
	// foot often sends only a release event (…;3u) for Ctrl+O; MC ignores that but we must accept it.
	mod := modEnc - 1
	masked := mod &^ (modCaps | modNum)
	return masked == modCtrl
}
