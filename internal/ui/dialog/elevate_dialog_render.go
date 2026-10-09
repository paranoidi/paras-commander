package dialog

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// DrawElevateDialog paints the "retry as root" prompt.
func DrawElevateDialog(screen tcell.Screen, layout Layout, state ElevateDialogState, styles theme.Theme) {
	if !state.Open {
		return
	}
	width := min(layout.Width-4, 64)
	if width < 44 {
		width = min(44, layout.Width-2)
	}
	rows := 5 // headline, path, separator, label+input or hint
	if state.BadPassword {
		rows++
	}
	if !state.HasInput() {
		rows--
	}
	height := rows + 4 // borders, blank row above the buttons, button row
	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, "Permission denied", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	text := styles.DialogText.Background(dbg)
	x, w := draw.DialogTextX(rect), draw.DialogContentWidth(rect)

	y := rect.Y + 1
	primitive.Text(screen, x, y, w, primitive.TruncateRight(elevateHeadline(state.Op), w), text)
	y++
	primitive.Text(screen, x, y, w, primitive.TruncateMiddle(state.Path, w), text)
	y++
	if state.BadPassword {
		primitive.Text(screen, x, y, w, "Incorrect password.", dialogErrorTextStyle(styles, dbg))
		y++
	}
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	if state.HasInput() {
		primitive.Text(screen, x, y, w, "Password:", text)
		y++
		masked := state.Password
		masked.Value = strings.Repeat("*", utf8.RuneCountInString(state.Password.Value))
		drawInputField(screen, x, y, w, masked, state.FocusOnInput(), styles)
		y++
	} else {
		primitive.Text(screen, x, y, w, "Retry as root with sudo?", text)
		y++
	}
	y++ // blank row above the buttons
	base := state.ButtonBase()
	draw.DrawDialogButtonRowCentered(screen, rect, y, []draw.DialogButtonSpec{
		{Label: "Elevate", Shortcut: 'E', Focused: state.Focus == base},
		{Label: "Skip", Shortcut: 'S', Focused: state.Focus == base+1},
		{Label: "Cancel", Shortcut: 'C', Focused: state.Focus == base+2},
	}, styles)
}

func elevateHeadline(op string) string {
	if op == "" {
		return "Permission denied"
	}
	return strings.ToUpper(op[:1]) + op[1:] + " failed"
}
