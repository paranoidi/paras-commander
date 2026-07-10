package dialog

import (
	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func compareMergeDialogHeight(showSharedPrefix bool) int {
	// Destination: label + blank + [optional shared] + radio1 + path1 + radio2 + path2 + sep
	dest := 7
	if showSharedPrefix {
		dest++
	}
	// Transfer:  label + blank + 2 checkboxes + sep = 5
	// Operation: label + blank + 2 radios + sep = 5
	// preview + blank + button = 3
	y := 1 + dest + 5 + 5 + 3
	return y + 1 // inner bottom margin + bottom border
}

// DrawCompareMergeDialog paints the compare merge modal.
func DrawCompareMergeDialog(screen tcell.Screen, layout Layout, state CompareMergeDialogState, styles theme.Theme, userHomeDir string) {
	width := PreferredFormDialogWidth
	if width < 52 {
		width = 52
	}
	sharedPrefix, leftPath, rightPath := FormatCompareMergePaths(state.PrimaryPath, state.SecondaryPath, userHomeDir)
	rect := draw.CenteredDialogRect(layout, width, compareMergeDialogHeight(sharedPrefix != ""))
	borderStyle := draw.DrawDialogFrame(screen, rect, "Merge", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	pathIndent := 4
	pathAvailW := draw.DialogContentWidth(rect) - pathIndent
	if pathAvailW < 1 {
		pathAvailW = 1
	}
	pathStyle := styles.DialogText.Background(dbg)

	y := rect.Y + 1

	// Destination section
	primitive.Text(screen, draw.DialogTextX(rect), y, draw.DialogContentWidth(rect), "Destination:", pathStyle)
	y += 2
	if sharedPrefix != "" {
		primitive.Text(screen, draw.DialogTextX(rect), y, draw.DialogContentWidth(rect),
			"Shared: "+primitive.FitPathForWidth(sharedPrefix, draw.DialogContentWidth(rect)-len("Shared: ")), pathStyle)
		y++
	}
	draw.DrawDialogRadio(screen, draw.DialogOptionX(rect), y, "Left side", 'L', state.Direction == comparepkg.MergeTowardPrimary, state.Focus == 0, styles)
	y++
	primitive.Text(screen, draw.DialogTextX(rect)+pathIndent, y, pathAvailW, primitive.FitPathForWidth(leftPath, pathAvailW), pathStyle)
	y++
	draw.DrawDialogRadio(screen, draw.DialogOptionX(rect), y, "Right side", 'R', state.Direction == comparepkg.MergeTowardSecondary, state.Focus == 1, styles)
	y++
	primitive.Text(screen, draw.DialogTextX(rect)+pathIndent, y, pathAvailW, primitive.FitPathForWidth(rightPath, pathAvailW), pathStyle)
	y++
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	// Transfer section
	primitive.Text(screen, draw.DialogTextX(rect), y, draw.DialogContentWidth(rect), "Transfer:", pathStyle)
	y += 2
	draw.DrawDialogCheckbox(screen, draw.DialogOptionX(rect), y, "Missing files", 'M', state.CopyMissing, state.Focus == 2, styles)
	y++
	draw.DrawDialogCheckbox(screen, draw.DialogOptionX(rect), y, "Modified files (content differs)", 'F', state.CopyModified, state.Focus == 3, styles)
	y++
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	// Operation section
	primitive.Text(screen, draw.DialogTextX(rect), y, draw.DialogContentWidth(rect), "Operation:", pathStyle)
	y += 2
	draw.DrawDialogRadio(screen, draw.DialogOptionX(rect), y, "Copy (keep source files)", 'K', !state.MoveMode, state.Focus == 4, styles)
	y++
	draw.DrawDialogRadio(screen, draw.DialogOptionX(rect), y, "Move (delete source after transfer)", 'D', state.MoveMode, state.Focus == 5, styles)
	y++
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	// Preview
	primitive.Text(screen, draw.DialogTextX(rect), y, draw.DialogContentWidth(rect), state.PreviewText, pathStyle)
	y++
	y++ // blank above buttons

	tform := NewCompareMergeDialogLinearForm()
	draw.DrawDialogButtonRowCentered(screen, rect, y, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: state.Focus == tform.OKIndex()},
		{Label: "Cancel", Shortcut: 'C', Focused: state.Focus == tform.CancelIndex()},
	}, styles)
}
