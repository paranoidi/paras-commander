package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

// DialogButtonSpec describes one rendered dialog button (label, Alt shortcut, focus).
type DialogButtonSpec = draw.DialogButtonSpec

// ScrollContentLen is the horizontal extent for overflow markers (see draw package).
var ScrollContentLen = draw.ScrollContentLen

// EnsureScrollInputVisible adjusts input scroll so the cursor stays visible (see draw package).
var EnsureScrollInputVisible = draw.EnsureScrollInputVisible

// AdjustScrollForCompletion updates scroll for ghost completion visibility (see draw package).
var AdjustScrollForCompletion = draw.AdjustScrollForCompletion

// EnsurePathInputScroll keeps the caret visible for path-shaped dialog inputs (see draw package).
var EnsurePathInputScroll = draw.EnsurePathInputScroll

// ShouldPreemptiveScrollRevealOnErase reports imminent deletion of the viewport's last visible rune.
var ShouldPreemptiveScrollRevealOnErase = draw.ShouldPreemptiveScrollRevealOnErase

// AdjustScrollRevealOnErase decreases scroll after deletions to reveal hidden prefix text.
var AdjustScrollRevealOnErase = draw.AdjustScrollRevealOnErase

// AccentGlyphStyle applies menu/dialog shortcut accent styling on top of a base row or label style.
func AccentGlyphStyle(base, accent tcell.Style) tcell.Style {
	return draw.AccentGlyphStyle(base, accent)
}

// DrawDialogHSeparator draws a horizontal rule inside a dialog frame (re-export for callers outside package dialog).
func DrawDialogHSeparator(screen tcell.Screen, rect geom.Rect, y int, borderStyle tcell.Style) {
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
}

// DrawDialogButtonRowCentered draws a centered row of dialog buttons (re-export for callers outside package dialog).
func DrawDialogButtonRowCentered(screen tcell.Screen, rect geom.Rect, y int, buttons []DialogButtonSpec, styles theme.Theme) {
	draw.DrawDialogButtonRowCentered(screen, rect, y, buttons, styles)
}

// DrawOKCancelButtonRow draws a centered OK/Cancel button row (re-export).
func DrawOKCancelButtonRow(screen tcell.Screen, rect geom.Rect, y int, okFocused, cancelFocused bool, styles theme.Theme) {
	draw.DrawOKCancelButtonRow(screen, rect, y, okFocused, cancelFocused, styles)
}

// OKCancelButtonSpecs returns standard OK/Cancel button specs (re-export).
func OKCancelButtonSpecs(okFocused, cancelFocused bool) []DialogButtonSpec {
	return draw.OKCancelButtonSpecs(okFocused, cancelFocused)
}
