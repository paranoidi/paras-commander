package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func DrawStashRestoreDialog(screen tcell.Screen, layout Layout, state StashRestoreDialogState, styles theme.Theme) {
	width := min(layout.Width-4, 72)
	if width < 44 {
		width = min(44, layout.Width-2)
	}
	height := 9
	rect := draw.CenteredDialogRect(layout, width, height)

	borderStyle := draw.DrawDialogFrame(screen, rect, "Stash restore", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	msg := "Panel has live selections and a non-empty stash."
	primitive.Text(screen, rect.X+2, rect.Y+1, rect.Width-4, msg, styles.DialogText.Background(dbg))
	msg2 := "Choose how to resolve:"
	primitive.Text(screen, rect.X+2, rect.Y+2, rect.Width-4, msg2, styles.DialogText.Background(dbg))

	sepY := rect.Y + 3
	draw.DrawDialogHSeparator(screen, rect, sepY, borderStyle)

	buttonSpecs := []struct {
		label    string
		shortcut rune
		idx      int
	}{
		{"Replace", 'R', 0},
		{"Merge", 'M', 1},
		{"Drop stash", 'D', 2},
		{"Drop all", 'A', 3},
	}
	row := make([]draw.DialogButtonSpec, len(buttonSpecs))
	for i, b := range buttonSpecs {
		row[i] = draw.DialogButtonSpec{
			Label:    b.label,
			Shortcut: b.shortcut,
			Focused:  state.Focus == b.idx,
		}
	}
	btnY := rect.Y + rect.Height - 2
	draw.DrawDialogHSeparator(screen, rect, btnY-1, borderStyle)
	draw.DrawDialogButtonRowCentered(screen, rect, btnY, row, styles)

	help := "Left/Right select  Enter confirm  Esc drop stash "
	primitive.Text(screen, rect.X+2, rect.Y+rect.Height-2, rect.Width-4, help, styles.DialogText.Background(dbg))
}
