package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func drawTransferDialog(screen tcell.Screen, layout Layout, state TransferDialogState, styles theme.Theme) {
	width := 60
	height := 10
	title := "Copy"
	if state.Kind == TransferKindMove {
		height = 8
		title = "Move/Rename"
	}
	if state.Phase == TransferPhaseSelfCopyRename {
		height = 9
		if state.Kind == TransferKindCopy {
			title = "Copy — New name"
		} else {
			title = "Move/Rename — New name"
		}
	}

	rect := centeredDialogRect(layout, width, height)
	borderStyle := drawDialogFrame(screen, rect, title, styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	if state.Phase == TransferPhaseSelfCopyRename {
		reason := "Cannot copy onto itself."
		if state.Kind == TransferKindMove {
			reason = "Cannot move onto itself."
		}
		primitive.Text(screen, rect.X+2, rect.Y+1, rect.Width-4, reason, styles.DialogText.Background(dbg))

		nameLabel := "New name:"
		primitive.Text(screen, rect.X+2, rect.Y+3, rect.Width-4, nameLabel, styles.DialogText.Background(dbg))

		inputY := rect.Y + 5
		inputWidth := rect.Width - 4
		drawInputField(screen, rect.X+2, inputY, inputWidth, state.SelfCopyNewName, state.FocusField == 0, styles)

		sepY := rect.Y + 6
		drawDialogHSeparator(screen, rect, sepY, borderStyle)

		tform := NewTransferDialogLinearForm(TransferDialogEffectiveNumContent(state))
		buttonY := rect.Y + rect.Height - 2
		drawDialogButtonRowCentered(screen, rect, buttonY, []DialogButtonSpec{
			{Label: "OK", Shortcut: 'O', Focused: state.FocusField == tform.OKIndex()},
			{Label: "Add paused", Shortcut: 'p', Focused: state.FocusField == tform.AddPausedIndex()},
			{Label: "Cancel", Shortcut: 'C', Focused: state.FocusField == tform.CancelIndex()},
		}, styles)
		return
	}

	destLabel := "Destination:"
	primitive.Text(screen, rect.X+2, rect.Y+1, rect.Width-4, destLabel, styles.DialogText.Background(dbg))

	inputY := rect.Y + 3
	inputWidth := rect.Width - 4
	drawSimpleDialogInput(screen, rect.X+2, inputY, inputWidth, state.Destination, state.FocusField == 0, styles)

	if state.Kind == TransferKindCopy {
		sep1Y := rect.Y + 4
		drawDialogHSeparator(screen, rect, sep1Y, borderStyle)

		drawDialogCheckbox(screen, rect.X+2, sep1Y+1, "Preserve permissions", 0, state.PreservePermissions, state.FocusField == 1, styles)
		drawDialogCheckbox(screen, rect.X+2, sep1Y+2, "Preserve timestamps", 0, state.PreserveTimestamps, state.FocusField == 2, styles)

		sep2Y := sep1Y + 3
		drawDialogHSeparator(screen, rect, sep2Y, borderStyle)

		tform := NewTransferDialogLinearForm(TransferDialogEffectiveNumContent(state))
		buttonY := rect.Y + rect.Height - 2
		drawDialogButtonRowCentered(screen, rect, buttonY, []DialogButtonSpec{
			{Label: "OK", Shortcut: 'O', Focused: state.FocusField == tform.OKIndex()},
			{Label: "Add paused", Shortcut: 'p', Focused: state.FocusField == tform.AddPausedIndex()},
			{Label: "Cancel", Shortcut: 'C', Focused: state.FocusField == tform.CancelIndex()},
		}, styles)
		return
	}

	sepY := rect.Y + 4
	drawDialogHSeparator(screen, rect, sepY, borderStyle)

	tform := NewTransferDialogLinearForm(TransferDialogEffectiveNumContent(state))
	buttonY := rect.Y + rect.Height - 2
	drawDialogButtonRowCentered(screen, rect, buttonY, []DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: state.FocusField == tform.OKIndex()},
		{Label: "Add paused", Shortcut: 'p', Focused: state.FocusField == tform.AddPausedIndex()},
		{Label: "Cancel", Shortcut: 'C', Focused: state.FocusField == tform.CancelIndex()},
	}, styles)
}
