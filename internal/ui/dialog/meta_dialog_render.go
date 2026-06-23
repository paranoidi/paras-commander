package dialog

import (
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func DrawMetaDialog(screen tcell.Screen, layout Layout, state MetaDialogState, styles theme.Theme) {
	n := len(state.Entries)
	form := NewDialogLinearForm(n)
	// top(1) + entries(n) + sep(1) + buttons(1) + bot(1) = n+4
	height := n + 4
	if height > layout.Height-2 {
		height = layout.Height - 2
	}
	if height < 5 {
		return
	}

	width := 56
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < 28 {
		return
	}

	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, "Meta", styles)

	primaryCol := draw.DialogOptionX(rect)
	y := rect.Y + 1

	shortcuts := MetaEntryShortcuts(state.Entries)
	for i, entry := range state.Entries {
		checked := i < len(state.Checked) && state.Checked[i]
		draw.DrawDialogCheckbox(screen, primaryCol, y, metaEntryDisplayLabel(entry), shortcuts[i], checked, state.Focus == i, styles)
		y++
	}

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	okFocused := state.Focus == form.OKIndex()
	cancelFocused := state.Focus == form.CancelIndex()
	draw.DrawDialogButtonRowCentered(screen, rect, y, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: okFocused},
		{Label: "Cancel", Shortcut: 'C', Focused: cancelFocused},
	}, styles)
}
