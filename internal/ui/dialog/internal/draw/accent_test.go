package draw

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestAccentGlyphStyleUsesAccentForegroundAndBaseBackground(t *testing.T) {
	base := tcell.StyleDefault.
		Foreground(tcell.NewRGBColor(1, 2, 3)).
		Background(tcell.NewRGBColor(4, 5, 6))
	accent := tcell.StyleDefault.
		Foreground(tcell.NewRGBColor(7, 8, 9)).
		Background(tcell.NewRGBColor(10, 11, 12)).
		Bold(true)

	out := AccentGlyphStyle(base, accent)
	oFg, oBg, attrs := out.Decompose()
	if oFg != tcell.NewRGBColor(7, 8, 9) {
		t.Fatalf("foreground = %v, want accent fg", oFg)
	}
	if oBg != tcell.NewRGBColor(4, 5, 6) {
		t.Fatalf("background = %v, want base bg (accent bg ignored)", oBg)
	}
	if attrs&tcell.AttrBold == 0 {
		t.Fatal("want bold from accent")
	}
}

func TestAccentGlyphStyleBoldFollowsAccentNotBase(t *testing.T) {
	base := tcell.StyleDefault.Background(tcell.NewRGBColor(1, 1, 1)).Bold(true)
	accent := tcell.StyleDefault.Foreground(tcell.NewRGBColor(2, 2, 2)).Bold(false)

	out := AccentGlyphStyle(base, accent)
	_, _, attrs := out.Decompose()
	if attrs&tcell.AttrBold != 0 {
		t.Fatal("accent not bold: result must not be bold")
	}
}
