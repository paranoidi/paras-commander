package theme

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestCutMarkStyleSwapsAnsiShades(t *testing.T) {
	for _, tc := range []struct{ in, want tcell.Color }{
		{tcell.ColorNavy, tcell.ColorBlue}, // blue -> bright_blue
		{tcell.ColorBlue, tcell.ColorNavy}, // bright_blue -> blue
		{tcell.ColorBlack, tcell.PaletteColor(8)},
		{tcell.ColorWhite, tcell.ColorSilver},
	} {
		fg, _, _ := CutMarkStyle(tcell.StyleDefault.Foreground(tc.in)).Decompose()
		if fg != tc.want {
			t.Fatalf("%v -> %v, want %v", tc.in, fg, tc.want)
		}
	}
}

func TestCutMarkStyleBoldAnsiDropsToNormalIntensity(t *testing.T) {
	// Bold blue already shows as bright blue on bold-is-bright terminals.
	for _, in := range []tcell.Color{tcell.ColorNavy, tcell.ColorBlue} {
		fg, _, attrs := CutMarkStyle(tcell.StyleDefault.Foreground(in).Bold(true)).Decompose()
		if fg != tcell.ColorNavy || attrs&tcell.AttrBold != 0 {
			t.Fatalf("bold %v -> %v bold=%v, want navy without bold", in, fg, attrs&tcell.AttrBold != 0)
		}
	}
}

func TestCutMarkStyleShadesRGB(t *testing.T) {
	dark := tcell.NewRGBColor(20, 40, 120)
	fg, _, _ := CutMarkStyle(tcell.StyleDefault.Foreground(dark)).Decompose()
	if r, g, b := fg.RGB(); r <= 20 || g <= 40 || b <= 120 {
		t.Fatalf("dark colour not lightened: %d %d %d", r, g, b)
	}
	light := tcell.NewRGBColor(230, 220, 200)
	fg, _, _ = CutMarkStyle(tcell.StyleDefault.Foreground(light)).Decompose()
	if r, g, b := fg.RGB(); r >= 230 || g >= 220 || b >= 200 {
		t.Fatalf("light colour not darkened: %d %d %d", r, g, b)
	}
}

func TestCutMarkStyleDefaultForegroundTogglesBold(t *testing.T) {
	_, _, attrs := CutMarkStyle(tcell.StyleDefault).Decompose()
	if attrs&tcell.AttrBold == 0 {
		t.Fatal("default foreground should turn bold")
	}
	_, _, attrs = CutMarkStyle(tcell.StyleDefault.Bold(true)).Decompose()
	if attrs&tcell.AttrBold != 0 {
		t.Fatal("bold default foreground should drop bold")
	}
}
