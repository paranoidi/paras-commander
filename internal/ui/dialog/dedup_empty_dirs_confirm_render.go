package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"

	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func DrawDedupEmptyDirsConfirmDialog(screen tcell.Screen, layout Layout, state DedupEmptyDirsConfirmState, styles theme.Theme) {
	width := 50
	height := 8
	rect := draw.CenteredDialogRect(layout, width, height)

	borderStyle := draw.DrawDialogFrame(screen, rect, "Remove Empty Directories", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	primitive.Text(screen, rect.X+2, rect.Y+1, rect.Width-4, "Remove directories left empty", styles.DialogText.Background(dbg))
	primitive.Text(screen, rect.X+2, rect.Y+2, rect.Width-4, "by this delete?", styles.DialogText.Background(dbg))

	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogHSeparator(screen, rect, buttonY-1, borderStyle)
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "Yes", Shortcut: 'Y', Focused: state.Focus == 0},
		{Label: "No", Shortcut: 'N', Focused: state.Focus == 1},
	}, styles)
}
