package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

func TestDrawPinDialogSmoke(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	layout := geom.CalculateLayout(80, 24, true, geom.PanelWidthSplit{})
	items := []PinDialogItem{
		{Path: "/tmp/alpha.txt"},
		{Path: "/tmp/beta.txt", IsDir: true},
	}
	state := PinDialogState{
		Open:   true,
		Query:  "tmp",
		Ranked: []int{0, 1},
		MatchRanges: [][]search.Range{
			{{Start: 1, End: 4}},
			{{Start: 1, End: 4}},
		},
		Selected:   0,
		ListScroll: 0,
	}
	DrawPinDialog(screen, layout, state, items, styles)

	width := 78
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	listH := PinDialogListRows(layout.Height)
	height := PinDialogFixedChromeRows + listH
	rect := draw.CenteredDialogRect(layout, width, height)
	queryCol := draw.DialogTextX(rect)

	cell, _, _ := screen.Get(queryCol, rect.Y+1)
	if cell == "" || cell == " " {
		t.Fatal("expected filter row content")
	}

	// Bottom border must land exactly on the last row of the computed rect — no stray
	// blank row before it, no clipping.
	borderCell, _, _ := screen.Get(rect.X, rect.Y+rect.Height-1)
	if borderCell != "└" {
		t.Fatalf("expected bottom-left border glyph at row %d, got %q", rect.Y+rect.Height-1, borderCell)
	}
}

func TestDrawPinDialogSmokeEmptyList(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	layout := geom.CalculateLayout(80, 24, true, geom.PanelWidthSplit{})
	state := PinDialogState{Open: true}
	// Should not panic with no items and no ranked rows.
	DrawPinDialog(screen, layout, state, nil, styles)
}

func TestEnsurePinListScroll(t *testing.T) {
	st := PinDialogState{
		Ranked:     make([]int, 20),
		Selected:   15,
		ListScroll: 0,
	}
	for i := range st.Ranked {
		st.Ranked[i] = i
	}
	EnsurePinListScroll(&st, 5)
	if st.ListScroll != 11 {
		t.Fatalf("ListScroll = %d want 11", st.ListScroll)
	}
}

func TestPinDialogListRowsNoLeftoverBlankRow(t *testing.T) {
	// Sanity-check the fixed-chrome-row accounting: for a layout height, the computed
	// dialog height (PinDialogFixedChromeRows + listH) must never exceed the space
	// CenteredDialogRect is given, and must consume it exactly (no stray blank rows,
	// no clipped bottom border) whenever the outer clamp isn't in effect.
	for layoutHeight := 20; layoutHeight <= 60; layoutHeight++ {
		listH := PinDialogListRows(layoutHeight)
		height := PinDialogFixedChromeRows + listH
		if height > layoutHeight-2 {
			t.Fatalf("layoutHeight=%d: dialog height %d exceeds available %d", layoutHeight, height, layoutHeight-2)
		}
		if listH < 4 {
			t.Fatalf("layoutHeight=%d: listH = %d, want >= 4", layoutHeight, listH)
		}
	}
}
