package dialog

import (
	"github.com/paranoidi/paras-commander/internal/ui/dialogdraw"
	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/theme"
)

func DrawConfigDialog(screen tcell.Screen, layout Layout, state ConfigDialogState, styles theme.Theme) {
	width := 54
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < 38 {
		return
	}

	const minHeight = 8
	height := minHeight
	if height > layout.Height-2 {
		height = layout.Height - 2
	}
	if height < minHeight {
		return
	}

	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, "Configuration", styles)

	leftCol := rect.X + 2
	y := rect.Y + 1
	draw.DrawDialogCheckbox(screen, leftCol, y, "Show file icons", 'f', state.ShowFileIcons, state.Focus == 0, styles)

	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogHSeparator(screen, rect, buttonY-1, borderStyle)
	okFocused := state.Focus == 1
	cancelFocused := state.Focus == 2
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: okFocused},
		{Label: "Cancel", Shortcut: 'C', Focused: cancelFocused},
	}, styles)
}
