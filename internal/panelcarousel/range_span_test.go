package panelcarousel

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestResolveColumnEntryRangeSpan(t *testing.T) {
	state := panel.State{Cursor: 2, RangeAnchor: "/tmp/amber", Entries: []localfs.Entry{
		{Name: "amber", Path: "/tmp/amber"}, {Name: "birch", Path: "/tmp/birch"},
		{Name: "cedar", Path: "/tmp/cedar"}, {Name: "dune", Path: "/tmp/dune"},
	}}
	styles := theme.Theme{PanelRowRange: tcell.StyleDefault.Background(tcell.ColorOlive)}
	cp := carouselColumnParams{
		Body:      BodyParams{Center: state, Styles: styles},
		C:         Column{Active: true},
		RangeSpan: state.PendingRange(),
	}
	for row, want := range []bool{true, true, false, false} { // the cursor row keeps its own style
		ce, ok := resolveCarouselColumnEntry(cp, row)
		if !ok {
			t.Fatalf("row %d: no entry", row)
		}
		_, bg, _ := ce.Style.Decompose()
		if got := bg == tcell.ColorOlive; got != want {
			t.Fatalf("row %d on range background = %v want %v", row, got, want)
		}
	}
}
