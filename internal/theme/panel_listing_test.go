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

func TestPanelListingInfoStyleKeepsRowBackground(t *testing.T) {
	th := Theme{
		PanelRowInfo: tcell.StyleDefault.Foreground(tcell.ColorGray),
		PanelRowFile: tcell.StyleDefault.Foreground(tcell.ColorWhite),
	}
	row := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorTeal)
	got := th.PanelListingInfoStyle(row, "", tcell.Style{})
	fg, bg, _ := got.Decompose()
	if fg != tcell.ColorGray || bg != tcell.ColorTeal {
		t.Fatalf("info style = fg %v bg %v, want gray on teal", fg, bg)
	}
}

// On a selected cursor row the info columns take the unselected cursor style, never the selected color.
func TestPanelListingInfoStyleOnSelectedCursorRowIsNotSelectedColor(t *testing.T) {
	th := Theme{
		PanelRowInfo:              tcell.StyleDefault.Foreground(tcell.ColorGray),
		PanelCursorActive:         tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorTeal),
		PanelActiveCursorSelected: tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.ColorTeal).Bold(true),
	}
	opts := PanelListingCursorOpts{FileListActive: true, Selected: true}
	row := th.PanelListingCursorStyle(tcell.StyleDefault, opts)
	info := th.PanelListingCursorInfoStyle(tcell.StyleDefault, opts)
	got := th.PanelListingInfoStyle(row, "panel.active.row.cursor.selected", info)
	if got != th.PanelCursorActive {
		t.Fatalf("selected cursor-row info style = %v, want the unselected cursor style %v", got, th.PanelCursorActive)
	}
}

// The inactive panel's cursor bar sets only a background: its info columns keep panel.row.info,
// not the entry-type color (a directory's blue).
func TestPanelListingInfoStyleOnInactiveCursorRowKeepsInfoColor(t *testing.T) {
	th := Theme{
		PanelRowInfo:        tcell.StyleDefault.Foreground(tcell.ColorGray),
		PanelCursorInactive: tcell.StyleDefault.Background(tcell.ColorBlack),
	}
	dir := tcell.StyleDefault.Foreground(tcell.ColorBlue).Bold(true)
	opts := PanelListingCursorOpts{}
	row := th.PanelListingCursorStyle(dir, opts)
	got := th.PanelListingInfoStyle(row, "panel.inactive.row.cursor", th.PanelListingCursorInfoStyle(dir, opts))
	want := tcell.StyleDefault.Foreground(tcell.ColorGray).Background(tcell.ColorBlack)
	if got != want {
		t.Fatalf("inactive cursor-row info style = %v, want %v", got, want)
	}
}
