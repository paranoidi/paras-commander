package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

const flattenDialogNumContent = 3

// DrawFlattenDialog paints the flatten confirmation modal.
func DrawFlattenDialog(screen tcell.Screen, layout Layout, state FlattenDialogState, styles theme.Theme) {
	width := PreferredFormDialogWidth
	height := 10
	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, "Flatten", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	primitive.Text(screen, draw.DialogTextX(rect), rect.Y+1, draw.DialogContentWidth(rect), "Destination:", styles.DialogText.Background(dbg))

	inputY := rect.Y + 3
	inputWidth := draw.DialogContentWidth(rect)
	rowFocused := state.FocusField == 0
	pickerFocused := rowFocused && state.DestSubFocus == FlattenDestSubFocusPicker
	destInvalid := state.DestPathInvalid && !state.DestPathCheckPending
	drawPathInputRow(screen, draw.DialogTextX(rect), inputY, inputWidth, state.Destination, rowFocused, pickerFocused, destInvalid, styles)

	sep1Y := rect.Y + 4
	draw.DrawDialogHSeparator(screen, rect, sep1Y, borderStyle)

	draw.DrawDialogCheckbox(screen, draw.DialogOptionX(rect), sep1Y+1, "Recursive flatten", 'R', state.Recursive, state.FocusField == 1, styles)
	draw.DrawDialogCheckbox(screen, draw.DialogOptionX(rect), sep1Y+2, "Remove empty directories", 'E', state.RemoveEmpty, state.FocusField == 2, styles)

	sep2Y := sep1Y + 3
	draw.DrawDialogHSeparator(screen, rect, sep2Y, borderStyle)

	tform := NewFlattenDialogLinearForm()
	buttonY := rect.Y + rect.Height - 2
	draw.DrawOKCancelButtonRow(screen, rect, buttonY, state.FocusField == tform.OKIndex(), state.FocusField == tform.CancelIndex(), styles)
}

// FlattenDialogLinearForm is the flatten dialog focus layout (destination + 2 checkboxes + OK/Cancel).
type FlattenDialogLinearForm = DialogTrailingButtonsForm

// NewFlattenDialogLinearForm returns the flatten dialog focus layout.
// Segments: destination(0) | recursive+removeEmpty(1,2) | buttons(3).
func NewFlattenDialogLinearForm() DialogTrailingButtonsForm {
	return NewDialogTrailingButtonsForm(flattenDialogNumContent, 2).WithSegments(0, 1, flattenDialogNumContent)
}

// FlattenDialogMoveFocus applies standard dialog navigation for the flatten dialog.
func FlattenDialogMoveFocus(focus int, key tcell.Key) (int, bool) {
	return NewFlattenDialogLinearForm().MoveFocus(focus, key)
}
