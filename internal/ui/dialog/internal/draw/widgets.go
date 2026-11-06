package draw

import (
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// RadioMarker is the padded radio marker (" ( ) " in the theme's icons) every radio row and
// Markers list paints from DialogOptionX: the leading space fills the margin cell so the glyph
// lands on DialogTextX.
func RadioMarker(styles theme.Theme, selected bool) string {
	return " " + styles.IconDialogRadio(selected) + " "
}

// CheckboxMarker is the padded checkbox marker (" [ ] " in the theme's icons), laid out like
// RadioMarker.
func CheckboxMarker(styles theme.Theme, checked bool) string {
	return " " + styles.IconDialogCheckbox(checked) + " "
}

// TriStateCheckboxMarker is CheckboxMarker for a tri-state checkbox: mixed wins over checked.
func TriStateCheckboxMarker(styles theme.Theme, checked, mixed bool) string {
	if mixed {
		return " " + styles.IconDialogCheckboxMixed() + " "
	}
	return CheckboxMarker(styles, checked)
}

// CheckboxText returns the ASCII marker+label string for width calculations.
// Always uses ASCII markers unconditionally; DrawDialogCheckbox renders the actual icons from the theme.
// Since ASCII markers are always the same width or wider than icon icons, this remains a safe upper bound.
func CheckboxText(label string, checked bool) string {
	if checked {
		return "[x] " + label
	}
	return "[ ] " + label
}

// RadioText returns the ASCII marker+label string for width calculations.
// Always uses ASCII markers unconditionally; DrawDialogRadio renders the actual icons from the theme.
// Since ASCII markers are always the same width or wider than icon icons, this remains a safe upper bound.
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
	disabled bool,
	styles theme.Theme,
) {
	markerStyle, style := styles.DialogOptionMarkerStyles(focused, checked)
	accent := styles.DialogAccent
	if disabled {
		style = styles.DialogOptionDisabledStyle()
		markerStyle, accent = style, style
	}
	marker := CheckboxMarker(styles, checked)
	primitive.Text(screen, x, y, utf8.RuneCountInString(marker), marker, markerStyle)
	drawDialogItem(screen, x+utf8.RuneCountInString(marker), y, label, shortcut, style, accent)
}

func DrawDialogRadio(
	screen tcell.Screen,
	x int,
	y int,
	label string,
	shortcut rune,
	selected bool,
	focused bool,
	disabled bool,
	styles theme.Theme,
) {
	markerStyle, style := styles.DialogOptionMarkerStyles(focused, selected)
	accent := styles.DialogAccent
	if disabled {
		style = styles.DialogOptionDisabledStyle()
		markerStyle, accent = style, style
	}
	marker := RadioMarker(styles, selected)
	primitive.Text(screen, x, y, utf8.RuneCountInString(marker), marker, markerStyle)
	drawDialogItem(screen, x+utf8.RuneCountInString(marker), y, label, shortcut, style, accent)
}

func drawDialogItem(screen tcell.Screen, x, y int, label string, shortcut rune, baseStyle, accentStyle tcell.Style) {
	col := 0
	highlighted := false
	for _, r := range label {
		style := baseStyle
		if !highlighted && unicode.ToLower(r) == unicode.ToLower(shortcut) {
			style = AccentIconStyle(baseStyle, accentStyle)
			highlighted = true
		}
		screen.SetContent(x+col, y, r, nil, style)
		col++
	}
}

// DialogOptionWidth is the cell width of a checkbox or radio row drawn by DrawDialogCheckbox /
// DrawDialogRadio for label: the " (*) " / " [x] " marker (ASCII width, an upper bound for icon
// markers) plus the label.
func DialogOptionWidth(label string) int {
	return 5 + utf8.RuneCountInString(label)
}
