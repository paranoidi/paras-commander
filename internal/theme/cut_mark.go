package theme

import "github.com/gdamore/tcell/v2"

// cutMarkShade is how far an RGB foreground moves toward white (dark colours) or black
// (light colours) for CutMarkStyle.
const cutMarkShade = 0.4

// CutMarkStyle returns st with its foreground swapped for the counterpart shade, marking where
// shortened text was cut without spending a cell: the 8 standard ANSI colours swap with their
// bright twins (blue ↔ bright_blue), other colours lighten or darken, and a terminal-default
// foreground toggles bold instead.
func CutMarkStyle(st tcell.Style) tcell.Style {
	fg, _, attrs := st.Decompose()
	switch {
	case !fg.Valid():
		return st.Bold(attrs&tcell.AttrBold == 0)
	case !fg.IsRGB() && fg-tcell.ColorValid < 16:
		return st.Foreground(tcell.PaletteColor(int(fg-tcell.ColorValid) ^ 8))
	}
	r, g, b := fg.RGB()
	if r < 0 {
		return st.Bold(attrs&tcell.AttrBold == 0)
	}
	toward := int32(255)
	if (299*r+587*g+114*b)/1000 >= 128 {
		toward = 0
	}
	shade := func(c int32) int32 { return c + int32(float64(toward-c)*cutMarkShade) }
	return st.Foreground(tcell.NewRGBColor(shade(r), shade(g), shade(b)))
}
