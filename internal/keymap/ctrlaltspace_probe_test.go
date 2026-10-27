package keymap

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestCtrlAltSpaceEncodings covers the shapes tcell reports for Ctrl+Alt+Space: CSI-u
// (Alt+Ctrl+' ') and legacy ESC NUL, which tcell decodes as Alt+Ctrl+'`' (e.g. under tmux 3.4).
func TestCtrlAltSpaceEncodings(t *testing.T) {
	want, err := ParseKey("C-M-space")
	if err != nil {
		t.Fatal(err)
	}
	cases := []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModCtrl|tcell.ModAlt),
		tcell.NewEventKey(tcell.KeyRune, '`', tcell.ModCtrl|tcell.ModAlt),
		tcell.NewEventKey(tcell.KeyCtrlSpace, 0, tcell.ModCtrl|tcell.ModAlt),
	}
	for _, ev := range cases {
		if got := CanonicalChord(EventChord(ev)); got != want {
			t.Errorf("%s: chord %+v, want %+v", ev.Name(), got, want)
		}
	}
}
