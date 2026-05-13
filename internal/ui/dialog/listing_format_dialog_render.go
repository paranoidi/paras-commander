package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func DrawListingFormatDialog(screen tcell.Screen, layout Layout, state ListingFormatDialogState, styles theme.Theme) {
	width := 44
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < 28 {
		return
	}
	// top(1) + radio(3) + sep(1) + buttons(1) + bot(1) = 7
	const minHeight = 7
	height := minHeight
	if height > layout.Height-2 {
		height = layout.Height - 2
	}
	if height < minHeight {
		return
	}

	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, "Listing format", styles)

	leftCol := rect.X + 2
	y := rect.Y + 1

	lf := panel.EffectiveListFormat(state.ListFormat)
	radios := panel.ListFormatDialogRadios()
	for i, r := range radios {
		draw.DrawDialogRadio(screen, leftCol, y, r.Label, r.Shortcut, lf == r.Format, state.Focus == i, styles)
		y++
	}

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	okFocused := state.Focus == 3
	cancelFocused := state.Focus == 4
	draw.DrawDialogButtonRowCentered(screen, rect, y, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: okFocused},
		{Label: "Cancel", Shortcut: 'C', Focused: cancelFocused},
	}, styles)
}
