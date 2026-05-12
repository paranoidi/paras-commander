package keymap

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// ParseKey parses a single key chord string. Multi-stroke forms (containing a space) are rejected.
func ParseKey(s string) (Chord, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Chord{}, fmt.Errorf("empty key")
	}
	if strings.ContainsAny(s, " \t") {
		return Chord{}, fmt.Errorf("multi-stroke keys are not supported: %q", s)
	}

	mod := tcell.ModMask(0)
	rest := s

	for {
		if strings.HasPrefix(rest, "S-") || strings.HasPrefix(rest, "s-") {
			mod |= tcell.ModShift
			rest = rest[2:]
			continue
		}
		if strings.HasPrefix(rest, "C-") || strings.HasPrefix(rest, "c-") {
			mod |= tcell.ModCtrl
			rest = rest[2:]
			continue
		}
		if strings.HasPrefix(rest, "M-") || strings.HasPrefix(rest, "m-") {
			mod |= tcell.ModAlt
			rest = rest[2:]
			continue
		}
		break
	}

	if rest == "" {
		return Chord{}, fmt.Errorf("key missing after modifiers in %q", s)
	}

	restLower := strings.ToLower(rest)

	if len(restLower) >= 2 && restLower[0] == 'f' {
		if k, ok := parseFKey(restLower); ok {
			return Chord{Key: k, Mod: mod}, nil
		}
	}

	if namedKey, ok := namedKeys[restLower]; ok {
		if strings.EqualFold(rest, "tab") && mod&tcell.ModShift != 0 {
			return Chord{Key: tcell.KeyBacktab, Mod: mod &^ tcell.ModShift}, nil
		}
		return Chord{Key: namedKey, Mod: mod}, nil
	}

	if restLower == "space" {
		return Chord{Key: tcell.KeyRune, Rune: ' ', Mod: mod}, nil
	}

	if mod&tcell.ModCtrl != 0 && len(rest) == 1 {
		k := ctrlLetterKey(unicode.ToLower(rune(rest[0])))
		if k != tcell.KeyRune {
			return Chord{Key: k, Mod: mod &^ tcell.ModCtrl}, nil
		}
	}

	// Support single printable rune (handles multi-byte UTF-8 like §, ~, etc.)
	restRunes := []rune(rest)
	if len(restRunes) == 1 {
		r := restRunes[0]
		if unicode.IsPrint(r) {
			return Chord{Key: tcell.KeyRune, Rune: r, Mod: mod}, nil
		}
	}

	return Chord{}, fmt.Errorf("unknown key %q", s)
}

func parseFKey(lower string) (tcell.Key, bool) {
	if len(lower) < 2 || lower[0] != 'f' {
		return 0, false
	}
	numStr := lower[1:]
	for _, c := range numStr {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	if numStr == "" {
		return 0, false
	}
	var n int
	for _, c := range numStr {
		n = n*10 + int(c-'0')
	}
	if n < 1 || n > 12 {
		return 0, false
	}
	return tcell.KeyF1 + tcell.Key(n-1), true
}

func ctrlLetterKey(r rune) tcell.Key {
	if r < 'a' || r > 'z' {
		return tcell.KeyRune
	}
	return tcell.KeyCtrlA + tcell.Key(r-'a')
}

var namedKeys = map[string]tcell.Key{
	"up":        tcell.KeyUp,
	"down":      tcell.KeyDown,
	"left":      tcell.KeyLeft,
	"right":     tcell.KeyRight,
	"pgup":      tcell.KeyPgUp,
	"pgdn":      tcell.KeyPgDn,
	"home":      tcell.KeyHome,
	"end":       tcell.KeyEnd,
	"tab":       tcell.KeyTab,
	"enter":     tcell.KeyEnter,
	"esc":       tcell.KeyEsc,
	"escape":    tcell.KeyEsc,
	"backspace": tcell.KeyBackspace,
	"insert":    tcell.KeyInsert,
	"delete":    tcell.KeyDelete,
}
