package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func DrawConfigDialog(screen tcell.Screen, layout Layout, state ConfigDialogState, styles theme.Theme) {
	width := 54
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < 38 {
		return
	}

	const minHeight = 14
	height := minHeight
	if height > layout.Height-2 {
		height = layout.Height - 2
	}
	if height < minHeight {
		return
	}

	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, "Configuration", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	leftCol := rect.X + 2
	y := rect.Y + 1
	draw.DrawDialogCheckbox(screen, leftCol, y, "Show file icons", 'f', state.ShowFileIcons, state.Focus == 0, styles)
	y++
	draw.DrawDialogCheckbox(screen, leftCol, y, "Zoom active panel", 'z', state.ZoomActivePanel, state.Focus == 1, styles)
	y++
	draw.DrawDialogCheckbox(screen, leftCol, y, "Shrunken shows only name", 's', state.ShrunkenShowsNameOnly, state.Focus == 2, styles)
	y++
	draw.DrawDialogCheckbox(screen, leftCol, y, "Center scrolling", 'e', state.CenterScrolling, state.Focus == 3, styles)
	y++
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	primitive.Text(screen, leftCol, y, rect.Width-4, "Default listing format", styles.DialogText.Background(dbg))
	y++

	lf := panel.EffectiveListFormat(state.ListFormat)
	radios := panel.ListFormatDialogRadios()
	for i, r := range radios {
		draw.DrawDialogRadio(screen, leftCol, y, r.Label, r.Shortcut, lf == r.Format, state.Focus == 4+i, styles)
		y++
	}

	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogHSeparator(screen, rect, buttonY-1, borderStyle)
	okFocused := state.Focus == 7
	cancelFocused := state.Focus == 8
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: okFocused},
		{Label: "Cancel", Shortcut: 'C', Focused: cancelFocused},
	}, styles)
}
