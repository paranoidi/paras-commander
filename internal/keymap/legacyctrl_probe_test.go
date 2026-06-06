package keymap

import (
	"github.com/gdamore/tcell/v2"
	"testing"
)

func TestLegacyCtrlCodesWithoutMod(t *testing.T) {
	m, _ := Default()
	cases := []struct {
		key    tcell.Key
		wantID string
	}{
		{10, ActionJobsOpen},
		{17, ActionJobsAnswerBlocker},
		{6, ActionPanelFindDialog},
	}
	for _, c := range cases {
		ev := tcell.NewEventKey(c.key, 0, tcell.ModNone)
		ch := CanonicalChord(EventChord(ev))
		id, ok := m.Lookup(ev)
		t.Logf("key=%d chord=%+v -> %q %v", c.key, ch, id, ok)
		if !ok || id != c.wantID {
			t.Errorf("key=%d = %q %v, want %q", c.key, id, ok, c.wantID)
		}
	}
}
