package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func DrawTransferDialog(screen tcell.Screen, layout Layout, state TransferDialogState, styles theme.Theme) {
	width := PreferredFormDialogWidth
	height := 10
	title := "Copy"
	if state.Kind == TransferKindMove {
		height = 8
		title = "Move"
	}
	if state.Phase == TransferPhaseSelfCopyRename {
		height = 9
		if state.Kind == TransferKindCopy {
			title = "Copy — New name"
		} else {
			title = "Move — New name"
		}
	}

	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, title, styles)
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
		draw.DrawDialogHSeparator(screen, rect, sepY, borderStyle)

		tform := NewTransferDialogLinearForm(TransferDialogEffectiveNumContent(state))
		buttonY := rect.Y + rect.Height - 2
		draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
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
	rowFocused := state.FocusField == 0
	pickerFocused := rowFocused && state.DestSubFocus == TransferDestSubFocusPicker
	destInvalid := state.Phase == TransferPhaseDestination && state.DestPathInvalid && !state.DestPathCheckPending
	drawPathInputRow(screen, rect.X+2, inputY, inputWidth, state.Destination, rowFocused, pickerFocused, destInvalid, styles)

	if state.Kind == TransferKindCopy {
		sep1Y := rect.Y + 4
		draw.DrawDialogHSeparator(screen, rect, sep1Y, borderStyle)

		// One cell left of labels/fields so "[ ]" aligns with other dialog content (see mass rename).
		draw.DrawDialogCheckbox(screen, rect.X+1, sep1Y+1, "Preserve permissions", 'r', state.PreservePermissions, state.FocusField == 1, styles)
		draw.DrawDialogCheckbox(screen, rect.X+1, sep1Y+2, "Preserve timestamps", 't', state.PreserveTimestamps, state.FocusField == 2, styles)

		sep2Y := sep1Y + 3
		draw.DrawDialogHSeparator(screen, rect, sep2Y, borderStyle)

		tform := NewTransferDialogLinearForm(TransferDialogEffectiveNumContent(state))
		buttonY := rect.Y + rect.Height - 2
		draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
			{Label: "OK", Shortcut: 'O', Focused: state.FocusField == tform.OKIndex()},
			{Label: "Add paused", Shortcut: 'p', Focused: state.FocusField == tform.AddPausedIndex()},
			{Label: "Cancel", Shortcut: 'C', Focused: state.FocusField == tform.CancelIndex()},
		}, styles)
		return
	}

	sepY := rect.Y + 4
	draw.DrawDialogHSeparator(screen, rect, sepY, borderStyle)

	tform := NewTransferDialogLinearForm(TransferDialogEffectiveNumContent(state))
	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: state.FocusField == tform.OKIndex()},
		{Label: "Add paused", Shortcut: 'p', Focused: state.FocusField == tform.AddPausedIndex()},
		{Label: "Cancel", Shortcut: 'C', Focused: state.FocusField == tform.CancelIndex()},
	}, styles)
}
