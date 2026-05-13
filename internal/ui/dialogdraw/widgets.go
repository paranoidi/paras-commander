package draw

import (
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// CheckboxText returns the marker+label string used for width calculations (matches draw row text).
func CheckboxText(label string, checked bool) string {
	if checked {
		return "[x] " + label
	}
	return "[ ] " + label
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
	style := DialogOptionRowStyle(focused, checked, styles)
	marker := " [ ] "
	if checked {
		marker = " [x] "
	}
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
	style := DialogOptionRowStyle(focused, selected, styles)
	marker := " ( ) "
	if selected {
		marker = " (*) "
	}
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

// DialogOptionRowStyle returns the theme style for a dialog option row (radio/checkbox).
func DialogOptionRowStyle(focused, selected bool, styles theme.Theme) tcell.Style {
	if focused {
		return styles.DialogOptionActive
	}
	if selected {
		return styles.DialogOptionSelected
	}
	return styles.DialogOptionInactive
}
