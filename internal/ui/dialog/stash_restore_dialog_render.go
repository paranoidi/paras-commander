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
	height := 7
	rect := draw.CenteredDialogRect(layout, width, height)

	borderStyle := draw.DrawDialogFrame(screen, rect, "Stash restore", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	textX, textW := draw.DialogTextX(rect), draw.DialogContentWidth(rect)
	textStyle := styles.DialogText.Background(dbg)

	y := rect.Y + 1
	primitive.Text(screen, textX, y, textW, "Panel has live selections and a non-empty stash.", textStyle)
	y++
	primitive.Text(screen, textX, y, textW, "Choose how to resolve:", textStyle)
	y++
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	y++ // surface-only blank row above buttons

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
	draw.DrawDialogButtonRowCentered(screen, rect, y, row, styles)
}
