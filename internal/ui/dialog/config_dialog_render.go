package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func DrawConfigDialog(screen tcell.Screen, layout Layout, state ConfigDialogState, styles theme.Theme) {
	const (
		width     = 54
		minWidth  = 38
		minHeight = 21
	)
	rect, ok := draw.ClampCenteredDialogRect(layout, width, minHeight, minWidth, minHeight)
	if !ok {
		return
	}
	borderStyle := draw.DrawDialogFrame(screen, rect, "Configuration", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	leftCol := draw.DialogTextX(rect)
	optionCol := draw.DialogOptionX(rect)
	y := rect.Y + 1
	primitive.Text(screen, leftCol, y, rect.Width-4, "View options:", styles.DialogText.Background(dbg))
	y++
	y++
	draw.DrawDialogCheckbox(screen, optionCol, y, "Show file icons", 'f', state.ShowFileIcons, state.Focus == 0, styles)
	y++
	draw.DrawDialogCheckbox(screen, optionCol, y, "Zoom active panel", 'z', state.ZoomActivePanel, state.Focus == 1, styles)
	y++
	draw.DrawDialogCheckbox(screen, optionCol, y, "Shrunken shows only name", 's', state.ShrunkenShowsNameOnly, state.Focus == 2, styles)
	y++
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	primitive.Text(screen, leftCol, y, rect.Width-4, "Scroll mode:", styles.DialogText.Background(dbg))
	y++
	y++

	scrollMode := panel.EffectiveScrollMode(state.ScrollMode)
	scrollRadios := panel.ScrollModeDialogRadios()
	for i, r := range scrollRadios {
		draw.DrawDialogRadio(screen, optionCol, y, r.Label, r.Shortcut, scrollMode == r.Mode, state.Focus == 3+i, styles)
		y++
	}
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	primitive.Text(screen, leftCol, y, rect.Width-4, "Default listing format:", styles.DialogText.Background(dbg))
	y++
	y++

	lf := panel.EffectiveListFormat(state.ListFormat)
	listRadios := panel.ListFormatDialogRadios()
	for i, r := range listRadios {
		draw.DrawDialogRadio(screen, optionCol, y, r.Label, r.Shortcut, lf == r.Format, state.Focus == 6+i, styles)
		y++
	}

	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogHSeparator(screen, rect, buttonY-1, borderStyle)
	okFocused := state.Focus == 9
	cancelFocused := state.Focus == 10
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: okFocused},
		{Label: "Cancel", Shortcut: 'C', Focused: cancelFocused},
	}, styles)
}
