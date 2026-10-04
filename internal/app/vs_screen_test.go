package app

import (
	"slices"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestVSStripScreen(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(10, 5)
	s := vsStripScreen{sim}
	st := tcell.StyleDefault

	in := []rune{0xFE0E}
	s.SetContent(0, 0, '⚪', in, st)
	s.SetContent(0, 1, ' ', []rune{0xFE0F}, st)
	s.SetContent(0, 2, 0xFE0F, nil, st)
	if !slices.Equal(in, []rune{0xFE0E}) {
		t.Fatalf("input combc mutated: %v", in)
	}
	for _, c := range []struct {
		y     int
		str   string
		width int
	}{{0, "⚪", 2}, {1, " ", 1}, {2, " ", 1}} {
		str, _, w := s.Get(0, c.y)
		if str != c.str || w != c.width {
			t.Errorf("row %d: got %q width %d, want %q width %d", c.y, str, w, c.str, c.width)
		}
	}
}
