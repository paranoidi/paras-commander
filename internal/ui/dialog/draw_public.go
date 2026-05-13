package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// DialogButtonSpec describes one rendered dialog button (label, Alt shortcut, focus).
type DialogButtonSpec = draw.DialogButtonSpec

// EnsureScrollInputVisible adjusts input scroll so the cursor stays visible (see draw package).
var EnsureScrollInputVisible = draw.EnsureScrollInputVisible

// AccentGlyphStyle applies menu/dialog shortcut accent styling on top of a base row or label style.
func AccentGlyphStyle(base, accent tcell.Style) tcell.Style {
	return draw.AccentGlyphStyle(base, accent)
}

// DrawDialogHSeparator draws a horizontal rule inside a dialog frame (re-export for callers outside package dialog).
func DrawDialogHSeparator(screen tcell.Screen, rect Rect, y int, borderStyle tcell.Style) {
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
}

// DrawDialogButtonRowCentered draws a centered row of dialog buttons (re-export for callers outside package dialog).
func DrawDialogButtonRowCentered(screen tcell.Screen, rect Rect, y int, buttons []DialogButtonSpec, styles theme.Theme) {
	draw.DrawDialogButtonRowCentered(screen, rect, y, buttons, styles)
}
