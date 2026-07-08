package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"

	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func DrawAmbiguousTransferDialog(screen tcell.Screen, layout Layout, state AmbiguousTransferState, styles theme.Theme) {
	width := 46
	height := 6
	rect := draw.CenteredDialogRect(layout, width, height)

	borderStyle := draw.DrawDialogFrame(screen, rect, "Ambiguous command", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	primitive.Text(screen, draw.DialogTextX(rect), rect.Y+1, draw.DialogContentWidth(rect), "Navigate to common root of selections?", styles.DialogText.Background(dbg))

	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogHSeparator(screen, rect, buttonY-2, borderStyle)
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: state.Focus == 0},
		{Label: "Cancel", Shortcut: 'C', Focused: state.Focus == 1},
	}, styles)
}
