package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"

	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func DrawUndoConfirmDialog(screen tcell.Screen, layout Layout, state UndoConfirmState, styles theme.Theme) {
	rect := draw.CenteredDialogRect(layout, 56, 7)

	borderStyle := draw.DrawDialogFrame(screen, rect, "Undo job", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	text := styles.DialogText.Background(dbg)
	primitive.Text(screen, rect.X+2, rect.Y+1, rect.Width-4, state.Line1, text)
	primitive.Text(screen, rect.X+2, rect.Y+2, rect.Width-4, state.Line2, text)

	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogHSeparator(screen, rect, buttonY-2, borderStyle)
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "Yes", Shortcut: 'Y', Focused: state.Focus == 0},
		{Label: "No", Shortcut: 'N', Focused: state.Focus == 1},
	}, styles)
}
