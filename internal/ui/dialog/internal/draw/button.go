package draw

import (
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// dialogButtonLabelRunePadding is extra width beyond utf8.RuneCountInString(label) for DrawDialogButton (spaces and brackets).
const dialogButtonLabelRunePadding = 6

func dialogButtonWidth(label string) int {
	return utf8.RuneCountInString(label) + dialogButtonLabelRunePadding
}

// DialogButtonWidth returns the rune width occupied by DrawDialogButton for label.
func DialogButtonWidth(label string) int {
	return dialogButtonWidth(label)
}

// DrawDialogButton renders a single button with its shortcut letter highlighted.
// shortcut is the letter inside label to highlight (e.g. 'O' for "OK").
// Output shape: space, "[", space, label, space, "]", space so theme backgrounds cover the chrome.
// Returns the rendered width in rune columns.
func DrawDialogButton(screen tcell.Screen, x, y int, label string, shortcut rune, focused, destructive bool, styles theme.Theme) int {
	var baseStyle tcell.Style
	switch {
	case focused && destructive:
		baseStyle = styles.DialogButtonActiveDestructive
	case focused:
		baseStyle = styles.DialogButtonActive
	default:
		baseStyle = styles.DialogButtonInactive
	}
	out := x
	primitive.Text(screen, out, y, 1, " ", baseStyle)
	out++
	primitive.Text(screen, out, y, 1, "[", baseStyle)
	out++
	primitive.Text(screen, out, y, 1, " ", baseStyle)
	out++

	highlighted := false
	for _, r := range label {
		style := baseStyle
		if !highlighted && (r == shortcut || r == unicode.ToUpper(shortcut) || r == unicode.ToLower(shortcut)) {
			style = AccentGlyphStyle(baseStyle, styles.DialogAccent)
			highlighted = true
		}
		screen.SetContent(out, y, r, nil, style)
		out++
	}

	primitive.Text(screen, out, y, 1, " ", baseStyle)
	out++
	primitive.Text(screen, out, y, 1, "]", baseStyle)
	out++
	primitive.Text(screen, out, y, 1, " ", baseStyle)
	out++

	return out - x
}
