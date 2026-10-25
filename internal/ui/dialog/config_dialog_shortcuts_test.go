package dialog

import (
	"strings"
	"testing"
	"unicode"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

type configDialogMnemonic struct {
	label    string
	shortcut rune
}

// configDialogMnemonics lists the dialog's widget mnemonics in spec order, read from the spec.
func configDialogMnemonics() []configDialogMnemonic {
	var out []configDialogMnemonic
	for _, g := range ConfigDialogView(ConfigDialogState{}, 80).Groups {
		for _, col := range g.Columns {
			for _, it := range col {
				for _, w := range append([]dlg.Item{it}, it.Cells...) {
					if w.Mnemonic != 0 {
						out = append(out, configDialogMnemonic{w.Label, w.Mnemonic})
					}
				}
			}
		}
	}
	return out
}

func labelContainsShortcut(label string, shortcut rune) bool {
	for _, r := range label {
		if unicode.ToLower(r) == unicode.ToLower(shortcut) {
			return true
		}
	}
	return false
}

func TestConfigDialogMnemonicsUniqueAndVisible(t *testing.T) {
	t.Parallel()
	seen := make(map[rune]string)
	for _, m := range configDialogMnemonics() {
		ch := unicode.ToLower(m.shortcut)
		if ch == 'o' || ch == 'c' {
			t.Fatalf("%q: shortcut %q reserved for OK/Cancel", m.label, string(m.shortcut))
		}
		if prev, ok := seen[ch]; ok {
			t.Fatalf("duplicate shortcut %q: %q and %q", string(ch), prev, m.label)
		}
		seen[ch] = m.label
		if !labelContainsShortcut(m.label, m.shortcut) {
			t.Fatalf("%q: shortcut %q does not appear in label %q", m.label, string(m.shortcut), m.label)
		}
	}
	if len(seen) != 13 {
		t.Fatalf("got %d mnemonics, want 13", len(seen))
	}
}

func TestConfigDialogMnemonicsExpectedSet(t *testing.T) {
	t.Parallel()
	want := "dzhineutrmpbs"
	var got strings.Builder
	for _, m := range configDialogMnemonics() {
		got.WriteRune(unicode.ToLower(m.shortcut))
	}
	if got.String() != want {
		t.Fatalf("shortcut letters = %q, want %q", got.String(), want)
	}
}
