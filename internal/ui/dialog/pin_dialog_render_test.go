package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
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
	DrawPinDialog(screen, layout, state, items, styles, uiscrollbar.StyleNone)

	p, ok := dlg.Place(layout.Width, layout.Height, PinDialogView(state, items, layout, styles, uiscrollbar.StyleNone))
	if !ok {
		t.Fatal("pin dialog does not fit")
	}
	rect := p.Rect
	queryCol := draw.DialogTextX(rect)

	cell, _, _ := screen.Get(queryCol, rect.Y+1)
	if cell == "" || cell == " " {
		t.Fatal("expected filter row content")
	}

	// Bottom border must land exactly on the last row of the computed rect — no stray
	// blank row before it, no clipping.
	borderCell, _, _ := screen.Get(rect.X, rect.Y+rect.Height-1)
	if borderCell != "└" {
		t.Fatalf("expected bottom-left border icon at row %d, got %q", rect.Y+rect.Height-1, borderCell)
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
	DrawPinDialog(screen, layout, state, nil, styles, uiscrollbar.StyleNone)
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

func TestPinDialogListRowsFitTheDialog(t *testing.T) {
	// The list height PinDialogListRows reports is the one drawn, at least 4 rows, in a dialog
	// that leaves a row free above and below it.
	for layoutHeight := 11; layoutHeight <= 60; layoutHeight++ {
		layout := Layout{Width: 80, Height: layoutHeight}
		listH := PinDialogListRows(layout)
		p, ok := dlg.Place(layout.Width, layout.Height, PinDialogView(PinDialogState{}, nil, layout, theme.Theme{}, uiscrollbar.StyleNone))
		if !ok {
			t.Fatalf("layoutHeight=%d: dialog does not fit", layoutHeight)
		}
		if p.Rect.Height > layoutHeight-2 {
			t.Fatalf("layoutHeight=%d: dialog height %d exceeds available %d", layoutHeight, p.Rect.Height, layoutHeight-2)
		}
		if list, _ := p.Item(PinList); list.Rect.Height != listH || listH < 4 {
			t.Fatalf("layoutHeight=%d: drawn list %d rows, PinDialogListRows %d, want equal and >= 4", layoutHeight, list.Rect.Height, listH)
		}
	}
	if got := PinDialogListRows(Layout{Width: 80, Height: 10}); got != 0 {
		t.Fatalf("layoutHeight=10: %d rows, want 0 (does not fit)", got)
	}
}

func TestDrawPinDialogScrollbarOnOverflow(t *testing.T) {
	items := make([]PinDialogItem, 40)
	ranked := make([]int, len(items))
	for i := range items {
		items[i] = PinDialogItem{Path: fmt.Sprintf("/tmp/lantern%02d", i), IsDir: true}
		ranked[i] = i
	}
	state := PinDialogState{Ranked: ranked}
	render := func(sb uiscrollbar.Style) string {
		screen := tcell.NewSimulationScreen("UTF-8")
		if err := screen.Init(); err != nil {
			t.Fatalf("Init: %v", err)
		}
		defer screen.Fini()
		screen.SetSize(80, 24)
		DrawPinDialog(screen, geom.CalculateLayout(80, 24, true, geom.PanelWidthSplit{}), state, items, theme.Default(), sb)
		var b strings.Builder
		for y := 0; y < 24; y++ {
			cell, _, _ := screen.Get(79-2, y) // right border column of the 76-wide dialog
			b.WriteString(cell)
		}
		return b.String()
	}
	if render(uiscrollbar.StyleNone) == render(uiscrollbar.StyleBar) {
		t.Fatal("bar scrollbar did not change the right border on an overflowing list")
	}
}
