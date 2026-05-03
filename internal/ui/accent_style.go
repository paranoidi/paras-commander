package ui

import "github.com/gdamore/tcell/v2"

// accentGlyphStyle applies menu/dialog shortcut accent styling on top of a base row or label style.
// Theme schema: only the accent entry’s foreground and bold flag affect the highlighted glyph;
// background and other attributes come from base.
func accentGlyphStyle(base, accent tcell.Style) tcell.Style {
	accFg, _, accAttrs := accent.Decompose()
	s := base.Foreground(accFg)
	return s.Bold((accAttrs & tcell.AttrBold) != 0)
}
