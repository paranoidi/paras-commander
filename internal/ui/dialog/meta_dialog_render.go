package dialog

import (
	"github.com/paranoidi/paras-commander/internal/ui/dialogdraw"
	"fmt"

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

	leftCol := rect.X + 2
	y := rect.Y + 1

	for i, entry := range state.Entries {
		label := entry.Name
		if entry.Description != "" {
			label = fmt.Sprintf("%s — %s", entry.Name, entry.Description)
		}
		var shortcut rune
		if len([]rune(entry.Name)) > 0 {
			shortcut = []rune(entry.Name)[0]
		}
		draw.DrawDialogRadio(screen, leftCol, y, label, shortcut, state.Selected == i, state.Focus == i, styles)
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
