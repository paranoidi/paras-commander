package keymap

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestCtrlQTerminalEncodings(t *testing.T) {
	m, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		ev   *tcell.EventKey
	}{
		{"KeyCtrlQ", tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModNone)},
		{"Rune0x11 no mod", tcell.NewEventKey(tcell.KeyRune, 0x11, tcell.ModNone)},
		{"Rune0x11 modctrl", tcell.NewEventKey(tcell.KeyRune, 0x11, tcell.ModCtrl)},
		{"Rune q modctrl", tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModCtrl)},
		{"Key17 no mod", tcell.NewEventKey(tcell.Key(17), 0, tcell.ModNone)},
		{"Key17 modctrl", tcell.NewEventKey(tcell.Key(17), 0, tcell.ModCtrl)},
	}
	for _, c := range cases {
		ev := c.ev
		ch := CanonicalChord(EventChord(ev))
		id, ok := m.Lookup(ev)
		t.Logf("%s key=%d rune=%q mod=%v chord=%+v -> %q %v", c.name, ev.Key(), ev.Rune(), ev.Modifiers(), ch, id, ok)
		if !ok || id != ActionJobsAnswerBlocker {
			t.Errorf("%s = %q %v, want jobs.answer-blocker", c.name, id, ok)
		}
	}
	ch, err := ParseKey("C-q")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ParseKey C-q = %+v", ch)
}
