package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawHistoryDialogSmoke(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	layout := CalculateLayout(80, 24, true)
	state := HistoryDialogState{
		Open:         true,
		PanelID:      LeftPanel,
		Paths:        []string{"/tmp/a", "/tmp/b"},
		CurrentIndex: 0,
		DisplayLines: []string{"* /tmp/a", "  /tmp/b"},
		Query:        "tmp",
		Ranked:       []int{0, 1},
		MatchRanges: [][]search.Range{
			{{Start: 2, End: 5}},
			{{Start: 2, End: 5}},
		},
		Selected:   0,
		ListScroll: 0,
		Focus:      0,
	}
	drawHistoryDialog(screen, layout, state, styles)
	cell, _, _ := screen.Get(4, layout.Menu.Height+4)
	if cell == "" || cell == " " {
		t.Fatal("expected filter row content")
	}
}

func TestEnsureHistoryListScroll(t *testing.T) {
	st := HistoryDialogState{
		Ranked:     make([]int, 20),
		Selected:   15,
		ListScroll: 0,
	}
	for i := range st.Ranked {
		st.Ranked[i] = i
	}
	EnsureHistoryListScroll(&st, 5)
	if st.ListScroll != 11 {
		t.Fatalf("ListScroll = %d want 11", st.ListScroll)
	}
}
