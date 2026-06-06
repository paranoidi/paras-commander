package keymap

import (
	"github.com/gdamore/tcell/v2"
	"testing"
)

func TestTcellKeyConstants(t *testing.T) {
	t.Logf("KeyCtrlA=%d KeyCtrlQ=%d KeyEnter=%d KeyRune=%d", tcell.KeyCtrlA, tcell.KeyCtrlQ, tcell.KeyEnter, tcell.KeyRune)
	ev := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	t.Logf("Enter ev key=%d rune=%q mod=%v", ev.Key(), ev.Rune(), ev.Modifiers())
	ch := CanonicalChord(EventChord(ev))
	t.Logf("Enter canonical %+v", ch)
	m, _ := Default()
	id, ok := m.Lookup(ev)
	t.Logf("Enter lookup %q %v", id, ok)
}
