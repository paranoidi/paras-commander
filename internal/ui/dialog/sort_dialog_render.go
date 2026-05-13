package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func DrawSortDialog(screen tcell.Screen, layout Layout, state SortDialogState, styles theme.Theme) {
	width := 56
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < 30 {
		return
	}
	// top(1) + radio(5) + checkboxes(2) + sep(1) + buttons(1) + bot(1) = 11
	const minHeight = 11
	height := minHeight
	if height > layout.Height-2 {
		height = layout.Height - 2
	}
	if height < minHeight {
		return
	}

	rect := centeredDialogRect(layout, width, height)
	borderStyle := DrawDialogFrame(screen, rect, "Sort order", styles)

	leftCol := rect.X + 2
	y := rect.Y + 1 // first content row

	// Radio list for sort mode (no blank row after title)
	modes := []struct {
		Mode     panel.SortMode
		Label    string
		Shortcut rune
	}{
		{panel.SortName, "Name", 'n'},
		{panel.SortExtension, "Extension", 'e'},
		{panel.SortSize, "Size", 's'},
		{panel.SortMtime, "Modify time", 'm'},
	}
	for i, m := range modes {
		DrawDialogRadio(screen, leftCol, y, m.Label, m.Shortcut, state.SortMode == m.Mode, state.Focus == i, styles)
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
		DrawDialogCheckbox(screen, leftCol, y, cb.label, cb.shortcut, cb.checked, cb.isFocus, styles)
		y++
	}

	// Separator
	DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	// Buttons (immediately after separator, no blank row)
	okFocused := state.Focus == 7
	cancelFocused := state.Focus == 8

	DrawDialogButtonRowCentered(screen, rect, y, []DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: okFocused},
		{Label: "Cancel", Shortcut: 'C', Focused: cancelFocused},
	}, styles)
}
