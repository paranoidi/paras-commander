package draw

import (
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// CheckboxText returns the ASCII marker+label string for width calculations.
// Always uses ASCII markers unconditionally; DrawDialogCheckbox renders the actual glyphs from the theme.
// Since ASCII markers are always the same width or wider than icon glyphs, this remains a safe upper bound.
func CheckboxText(label string, checked bool) string {
	if checked {
		return "[x] " + label
	}
	return "[ ] " + label
}

// RadioText returns the ASCII marker+label string for width calculations.
// Always uses ASCII markers unconditionally; DrawDialogRadio renders the actual glyphs from the theme.
// Since ASCII markers are always the same width or wider than icon glyphs, this remains a safe upper bound.
func RadioText(label string, selected bool) string {
	if selected {
		return " (*) " + label
	}
	return " ( ) " + label
}

func DrawDialogCheckbox(
	screen tcell.Screen,
	x int,
	y int,
	label string,
	shortcut rune,
	checked bool,
	focused bool,
	styles theme.Theme,
) {
	style := styles.DialogOptionRowStyle(focused, checked)
	marker := " " + styles.SymbolDialogCheckbox(checked) + " "
	primitive.Text(screen, x, y, utf8.RuneCountInString(marker), marker, style)
	drawDialogItem(screen, x+utf8.RuneCountInString(marker), y, label, shortcut, style, styles.DialogAccent)
}

func DrawDialogRadio(
	screen tcell.Screen,
	x int,
	y int,
	label string,
	shortcut rune,
	selected bool,
	focused bool,
	styles theme.Theme,
) {
	style := styles.DialogOptionRowStyle(focused, selected)
	marker := " " + styles.SymbolDialogRadio(selected) + " "
	primitive.Text(screen, x, y, utf8.RuneCountInString(marker), marker, style)
	drawDialogItem(screen, x+utf8.RuneCountInString(marker), y, label, shortcut, style, styles.DialogAccent)
}

func drawDialogItem(screen tcell.Screen, x, y int, label string, shortcut rune, baseStyle, accentStyle tcell.Style) {
	col := 0
	highlighted := false
	for _, r := range label {
		style := baseStyle
		if !highlighted && unicode.ToLower(r) == unicode.ToLower(shortcut) {
			style = AccentGlyphStyle(baseStyle, accentStyle)
			highlighted = true
		}
		screen.SetContent(x+col, y, r, nil, style)
		col++
	}
}
