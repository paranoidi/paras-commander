package dialog

import (
	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func DrawQuitConfirmDialog(screen tcell.Screen, layout Layout, state QuitConfirmState, styles theme.Theme) {
	width := 50
	height := 8
	rect := centeredDialogRect(layout, width, height)

	borderStyle := DrawDialogFrame(screen, rect, "Quit", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	msg := state.WarnLine1
	if msg == "" {
		msg = "Active jobs are queued or running."
	}
	primitive.Text(screen, rect.X+2, rect.Y+1, rect.Width-4, msg, styles.StatusWarn.Background(dbg))
	msg2 := state.WarnLine2
	if msg2 == "" {
		msg2 = "Quitting will interrupt these operations."
	}
	primitive.Text(screen, rect.X+2, rect.Y+2, rect.Width-4, msg2, styles.StatusWarn.Background(dbg))

	buttonY := rect.Y + rect.Height - 2
	DrawDialogHSeparator(screen, rect, buttonY-1, borderStyle)
	DrawDialogButtonRowCentered(screen, rect, buttonY, []DialogButtonSpec{
		{Label: "Stay", Shortcut: 'S', Focused: state.Focus == 0},
		{Label: "Quit Anyway", Shortcut: 'Q', Focused: state.Focus == 1},
	}, styles)
}
