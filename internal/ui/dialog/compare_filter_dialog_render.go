package dialog

import (
	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// DrawCompareFilterDialog paints the compare category filter picker modal.
func DrawCompareFilterDialog(screen tcell.Screen, layout Layout, state CompareFilterDialogState, styles theme.Theme) {
	const width = 32
	radios := state.Radios()
	height := len(radios) + 4 // border + radios + separator + buttons + border
	rect, ok := draw.ClampCenteredDialogRect(layout, width, height, 24, height)
	if !ok {
		return
	}
	borderStyle := draw.DrawDialogFrame(screen, rect, state.Title(), styles)

	y := rect.Y + 1

	for i, r := range radios {
		draw.DrawDialogCheckbox(screen, draw.DialogOptionX(rect), y, r.Label, r.Shortcut,
			comparepkg.FilterChecked(state.Filter, r.Filter), state.Focus == i, false, styles)
		y++
	}

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	draw.DrawOKCancelButtonRow(screen, rect, y, state.Focus == state.OKIndex(), state.Focus == state.CancelIndex(), styles)
}
