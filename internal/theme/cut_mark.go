package theme

import (
	"math"

	"github.com/gdamore/tcell/v2"
)

// An RGB cut mark moves its foreground toward white (dark colours) or black (light colours) by
// cutMarkShade, stepping further (by cutMarkShadeStep, up to all the way) and trying the other
// direction until the mark keeps cutMarkMinBG contrast against an RGB row background and
// cutMarkMinFG against the text it marks; when no shade passes, the shade closest to both wins. themes/import/import_alacritty.py (cut_mark_ok) checks
// the same rule on generated themes: keep the constants in sync.
const (
	cutMarkShade     = 0.4
	cutMarkShadeStep = 0.1
	cutMarkMinBG     = 3.0
	cutMarkMinFG     = 1.5
)

// CutMarkStyle returns st with its foreground swapped for the counterpart shade, marking where
// shortened text was cut without spending a cell: the 8 standard ANSI colours swap with their
// bright twins (blue ↔ bright_blue), other colours lighten or darken, and a terminal-default
// foreground toggles bold instead. A bold ANSI colour already renders bright on most terminals
// (bold-is-bright), so its mark is the normal-intensity twin without bold.
func CutMarkStyle(st tcell.Style) tcell.Style {
	fg, _, attrs := st.Decompose()
	switch {
	case !fg.Valid():
		return st.Bold(attrs&tcell.AttrBold == 0)
	case !fg.IsRGB() && fg-tcell.ColorValid < 16 && attrs&tcell.AttrBold != 0:
		return st.Foreground(tcell.PaletteColor(int(fg-tcell.ColorValid) &^ 8)).Bold(false)
	case !fg.IsRGB() && fg-tcell.ColorValid < 16:
		return st.Foreground(tcell.PaletteColor(int(fg-tcell.ColorValid) ^ 8))
	}
	r, g, b := fg.RGB()
	if r < 0 {
		return st.Bold(attrs&tcell.AttrBold == 0)
	}
	first, second := int32(255), int32(0)
	if (299*r+587*g+114*b)/1000 >= 128 {
		first, second = 0, 255
	}
	mark := func(toward int32, amount float64) tcell.Color {
		shade := func(c int32) int32 { return c + int32(float64(toward-c)*amount) }
		return tcell.NewRGBColor(shade(r), shade(g), shade(b))
	}
	_, bg, _ := st.Decompose()
	if br, _, _ := bg.RGB(); !bg.Valid() || br < 0 {
		return st.Foreground(mark(first, cutMarkShade))
	}
	best, bestScore := mark(first, cutMarkShade), 0.0
	for amount := cutMarkShade; amount <= 1+1e-9; amount += cutMarkShadeStep {
		for _, toward := range []int32{first, second} {
			c := mark(toward, amount)
			score := math.Min(contrastRatio(c, bg)/cutMarkMinBG, contrastRatio(c, fg)/cutMarkMinFG)
			if score >= 1 {
				return st.Foreground(c)
			}
			if score > bestScore {
				best, bestScore = c, score
			}
		}
	}
	return st.Foreground(best)
}

// contrastRatio is the WCAG contrast ratio of two RGB colours (1 to 21).
func contrastRatio(a, b tcell.Color) float64 {
	la, lb := relLuminance(a), relLuminance(b)
	return (math.Max(la, lb) + 0.05) / (math.Min(la, lb) + 0.05)
}

func relLuminance(c tcell.Color) float64 {
	r, g, b := c.RGB()
	lin := func(v int32) float64 {
		x := float64(v) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}
