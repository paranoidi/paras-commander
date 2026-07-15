package theme

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestPanelListingCursorStyleInheritsBaseWhenFGUnset(t *testing.T) {
	th := Theme{
		PanelCursorInactive: tcell.StyleDefault.Background(tcell.ColorBlack),
		PanelCursorActive:   tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorTeal),
	}
	base := tcell.StyleDefault.Foreground(tcell.ColorBlue).Bold(true)

	got := th.PanelListingCursorStyle(base, PanelListingCursorOpts{})
	fg, bg, attrs := got.Decompose()
	if fg != tcell.ColorBlue || bg != tcell.ColorBlack || attrs&tcell.AttrBold == 0 {
		t.Fatalf("inactive cursor = fg %v bg %v attrs %v, want base fg blue bold on cursor bg black", fg, bg, attrs)
	}

	got = th.PanelListingCursorStyle(base, PanelListingCursorOpts{FileListActive: true})
	fg, bg, _ = got.Decompose()
	if fg != tcell.ColorBlack || bg != tcell.ColorTeal {
		t.Fatalf("active cursor = fg %v bg %v, want theme cursor style unchanged", fg, bg)
	}
}
