package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func DrawHostKeyDialog(screen tcell.Screen, layout Layout, state HostKeyDialogState, styles theme.Theme) {
	if !state.Open {
		return
	}
	width := min(layout.Width-4, 72)
	if width < 44 {
		width = min(44, layout.Width-2)
	}
	height := 8
	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, "Host key", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	lines := []string{
		"Unknown host key for:",
		state.Host,
		"Type: " + state.KeyType,
		"SHA256: " + state.Fingerprint,
	}
	for i, line := range lines {
		if i >= 4 {
			break
		}
		primitive.Text(screen, rect.X+2, rect.Y+1+i, rect.Width-4, truncateStr(line, rect.Width-4), styles.DialogText.Background(dbg))
	}

	sepY := rect.Y + 5
	draw.DrawDialogHSeparator(screen, rect, sepY, borderStyle)

	buttons := []draw.DialogButtonSpec{
		{Label: "Accept", Shortcut: 'A', Focused: state.Focus == 0},
		{Label: "Save", Shortcut: 'S', Focused: state.Focus == 1},
		{Label: "Reject", Shortcut: 'R', Focused: state.Focus == 2},
	}
	btnY := rect.Y + rect.Height - 2
	draw.DrawDialogButtonRowCentered(screen, rect, btnY, buttons, styles)
}
