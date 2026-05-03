package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawBookmarkDialogSmoke(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	layout := CalculateLayout(80, 24, true)
	state := BookmarkDialogState{
		Open:    true,
		Query:   "pro",
		Entries: []BookmarkEntry{{Name: "proj", Path: "/tmp/x", Line: "proj : /tmp/x"}},
		Ranked:  []int{0},
		MatchRanges: [][]search.Range{
			{{Start: 0, End: 1}},
		},
		Selected:   0,
		ListScroll: 0,
		Focus:      0,
	}
	drawBookmarkDialog(screen, layout, state, styles)
	cell, _, _ := screen.Get(4, layout.Menu.Height+4)
	if cell == "" || cell == " " {
		t.Fatal("expected filter row content")
	}
}

func TestEnsureBookmarkListScroll(t *testing.T) {
	st := BookmarkDialogState{
		Ranked:     make([]int, 20),
		Selected:   15,
		ListScroll: 0,
	}
	for i := range st.Ranked {
		st.Ranked[i] = i
	}
	EnsureBookmarkListScroll(&st, 5)
	if st.ListScroll != 11 {
		t.Fatalf("ListScroll = %d want 11", st.ListScroll)
	}
}
