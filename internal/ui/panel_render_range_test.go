package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestPanelRowStyleRangeSpan(t *testing.T) {
	styles := theme.Theme{PanelRowFile: tcell.StyleDefault.Foreground(tcell.ColorWhite)}
	styles.PanelRowSelected = tcell.StyleDefault.Foreground(tcell.ColorYellow)
	styles.PanelRowRange = tcell.StyleDefault.Background(tcell.ColorOlive)
	state := panel.State{Cursor: 3, Entries: []localfs.Entry{
		{Name: "amber", Path: "/tmp/amber"}, {Name: "birch", Path: "/tmp/birch"},
		{Name: "cedar", Path: "/tmp/cedar"}, {Name: "dune", Path: "/tmp/dune"}, {Name: "ember", Path: "/tmp/ember"},
	}}
	state.RangeAnchor = "/tmp/birch"
	state.AddSelection("/tmp/cedar")
	ctx := PanelContext{RangeSpan: state.PendingRange()}
	bg := func(i int) tcell.Color {
		st, _ := panelRowStyle(state.Entries[i], i, state, ctx, styles)
		_, b, _ := st.Decompose()
		return b
	}
	for _, i := range []int{1, 2} {
		if bg(i) != tcell.ColorOlive {
			t.Fatalf("row %d not on the range background", i)
		}
	}
	for _, i := range []int{0, 3, 4} {
		if bg(i) == tcell.ColorOlive {
			t.Fatalf("row %d must not be on the range background", i)
		}
	}
	st, _ := panelRowStyle(state.Entries[2], 2, state, ctx, styles)
	if fg, _, _ := st.Decompose(); fg != tcell.ColorYellow {
		t.Fatalf("selected row in the span lost its foreground: %v", fg)
	}
	ctx.ChromeBlocked = true
	if bg(1) == tcell.ColorOlive {
		t.Fatal("blocked chrome must not paint the span")
	}
}
