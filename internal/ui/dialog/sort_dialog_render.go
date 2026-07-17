package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func DrawSortDialog(screen tcell.Screen, layout Layout, state SortDialogState, styles theme.Theme) {
	const (
		width     = 56
		minWidth  = 30
		minHeight = 11
	)
	rect, ok := draw.ClampCenteredDialogRect(layout, width, minHeight, minWidth, minHeight)
	if !ok {
		return
	}
	borderStyle := draw.DrawDialogFrame(screen, rect, "Sort order", styles)

	primaryCol := rect.X + 2
	y := rect.Y + 1 // first content row

	// Radio list for sort mode (no blank row after title)
	for i, m := range panel.SortDialogRadios() {
		draw.DrawDialogRadio(screen, primaryCol, y, m.Label, m.Shortcut, state.SortMode == m.Mode, state.Focus == i, styles)
		y++
	}

	// Checkboxes (immediately after radio, no blank row)
	for _, cb := range []struct {
		label    string
		shortcut rune
		checked  bool
		isFocus  bool
	}{
		{"Disk usage", 'u', state.DiskUsageIdleSizeSort, state.Focus == 4},
		{"Reverse", 'r', state.SortReverse, state.Focus == 5},
		{"Directories first", 'd', state.DirectoriesFirst, state.Focus == 6},
	} {
		draw.DrawDialogCheckbox(screen, primaryCol, y, cb.label, cb.shortcut, cb.checked, cb.isFocus, styles)
		y++
	}

	// Separator
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	// Buttons (immediately after separator, no blank row)
	okFocused := state.Focus == 7
	cancelFocused := state.Focus == 8

	draw.DrawOKCancelButtonRow(screen, rect, y, okFocused, cancelFocused, styles)
}
