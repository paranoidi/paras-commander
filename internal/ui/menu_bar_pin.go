package ui

import (
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// drawMenuBarPinBadge paints "<glyph> <count> " (styles.MenuBarAccent) at the start of the
// menu-bar jobs gap when at least one item is pinned, clipped to maxWidth. Returns the
// consumed cell width so the caller can shrink the jobs-gap span before painting it.
func drawMenuBarPinBadge(screen tcell.Screen, y, x, maxWidth, count int, styles theme.Theme) int {
	if count <= 0 || maxWidth <= 0 {
		return 0
	}
	text := styles.SymbolPin() + " " + strconv.Itoa(count) + " "
	w := runewidth.StringWidth(text)
	if w > maxWidth {
		w = maxWidth
	}
	primitive.Text(screen, x, y, w, text, styles.MenuBarAccent)
	return w
}
