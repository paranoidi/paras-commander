package dialog

import (
	"strings"
	"testing"
	"unicode"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

type configDialogMnemonic struct {
	label    string
	shortcut rune
}

func configDialogMnemonics() []configDialogMnemonic {
	var out []configDialogMnemonic
	out = append(out,
		configDialogMnemonic{"Show file icons", 'f'},
		configDialogMnemonic{"Zoom active panel", 'z'},
		configDialogMnemonic{"Shrunken shows only name", 's'},
	)
	for _, r := range panel.ScrollModeDialogRadios() {
		out = append(out, configDialogMnemonic{r.Label, r.Shortcut})
	}
	for _, r := range uiscrollbar.DialogRadios() {
		out = append(out, configDialogMnemonic{r.Label, r.Shortcut})
	}
	for _, r := range panel.ListFormatDialogRadios() {
		out = append(out, configDialogMnemonic{r.Label, r.Shortcut})
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
	if len(seen) != 12 {
		t.Fatalf("got %d mnemonics, want 12", len(seen))
	}
}

func TestConfigDialogMnemonicsExpectedSet(t *testing.T) {
	t.Parallel()
	want := "fzsietnurmpb"
	var got strings.Builder
	for _, m := range configDialogMnemonics() {
		got.WriteRune(unicode.ToLower(m.shortcut))
	}
	if got.String() != want {
		t.Fatalf("shortcut letters = %q, want %q", got.String(), want)
	}
}
