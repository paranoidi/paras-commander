package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
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

	optionCol := draw.DialogOptionX(rect)
	metaCol := optionCol + width/2
	metaWidth := rect.X + rect.Width - 1 - metaCol // stop one cell before the right border
	y := rect.Y + 1                                // first content row

	// Radio list for sort mode, with active meta columns (capped, see MetaCount) in a second
	// column on the same rows so the dialog never grows taller.
	builtinRadios := panel.SortDialogRadios()
	metaCount := state.MetaCount()
	for i, m := range builtinRadios {
		draw.DrawDialogRadio(screen, optionCol, y, m.Label, m.Shortcut, state.SortMode == m.Mode, state.Focus == i, styles)
		if i < metaCount {
			radio := state.MetaRadios[i]
			label := primitive.TruncateRight(radio.Title, metaWidth)
			selected := state.SortMode == panel.SortMeta && state.MetaColumn == radio.Name
			draw.DrawDialogRadio(screen, metaCol, y, label, 0, selected, state.Focus == len(builtinRadios)+i, styles)
		}
		y++
	}

	// Checkboxes (immediately after radios, no blank row)
	checkboxFocus := state.CheckboxFocus()
	for i, cb := range []struct {
		label    string
		shortcut rune
		checked  bool
	}{
		{"Disk usage", 'u', state.DiskUsageIdleSizeSort},
		{"Reverse", 'r', state.SortReverse},
		{"Directories first", 'd', state.DirectoriesFirst},
	} {
		draw.DrawDialogCheckbox(screen, optionCol, y, cb.label, cb.shortcut, cb.checked, state.Focus == checkboxFocus+i, false, styles)
		y++
	}

	// Separator
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	// Buttons (immediately after separator, no blank row)
	okFocused := state.Focus == state.OKFocus()
	cancelFocused := state.Focus == state.CancelFocus()

	draw.DrawOKCancelButtonRow(screen, rect, y, okFocused, cancelFocused, styles)
}
