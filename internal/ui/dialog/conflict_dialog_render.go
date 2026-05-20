package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func DrawConflictDialog(screen tcell.Screen, layout Layout, state ConflictDialogState, styles theme.Theme) {
	width := min(layout.Width-4, 76)
	if width < 40 {
		width = min(40, layout.Width-2)
	}
	height := 11
	rect := draw.CenteredDialogRect(layout, width, height)

	borderStyle := draw.DrawDialogFrame(screen, rect, "Conflict", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	warnLine := "Destination already exists."
	primitive.Text(screen, rect.X+2, rect.Y+1, rect.Width-4, warnLine, styles.MessageWarn.Background(dbg))
	srcLine := "Source: " + state.Source
	dstLine := "Dest:   " + state.Destination
	primitive.Text(screen, rect.X+2, rect.Y+2, rect.Width-4, truncateStr(srcLine, rect.Width-4), styles.DialogText.Background(dbg))
	primitive.Text(screen, rect.X+2, rect.Y+3, rect.Width-4, truncateStr(dstLine, rect.Width-4), styles.DialogText.Background(dbg))

	sepY := rect.Y + 4
	draw.DrawDialogHSeparator(screen, rect, sepY, borderStyle)

	buttonSpecs := []struct {
		label    string
		shortcut rune
		idx      int
	}{
		{"Overwrite", 'O', 0},
		{"Skip", 'S', 1},
		{"Overwrite All", 'A', 2},
		{"Skip All", 'L', 3},
		{"Cancel", 'C', 4},
	}

	row1 := []draw.DialogButtonSpec{
		{Label: buttonSpecs[0].label, Shortcut: buttonSpecs[0].shortcut, Focused: state.Focus == buttonSpecs[0].idx},
		{Label: buttonSpecs[1].label, Shortcut: buttonSpecs[1].shortcut, Focused: state.Focus == buttonSpecs[1].idx},
		{Label: buttonSpecs[2].label, Shortcut: buttonSpecs[2].shortcut, Focused: state.Focus == buttonSpecs[2].idx},
	}
	row2 := []draw.DialogButtonSpec{
		{Label: buttonSpecs[3].label, Shortcut: buttonSpecs[3].shortcut, Focused: state.Focus == buttonSpecs[3].idx},
		{Label: buttonSpecs[4].label, Shortcut: buttonSpecs[4].shortcut, Focused: state.Focus == buttonSpecs[4].idx},
	}

	btnRow1 := rect.Y + rect.Height - 4
	btnRow2 := rect.Y + rect.Height - 3
	draw.DrawDialogButtonRowCentered(screen, rect, btnRow1, row1, styles)
	draw.DrawDialogButtonRowCentered(screen, rect, btnRow2, row2, styles)

	help := "Left/Right select  Enter confirm  Esc cancel "
	primitive.Text(screen, rect.X+2, rect.Y+rect.Height-2, rect.Width-4, help, styles.DialogText.Background(dbg))
}

func truncateStr(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "~"
}
