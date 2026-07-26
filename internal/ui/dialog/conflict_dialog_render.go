package dialog

import (
	"fmt"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

const (
	conflictDialogLabelNew      = "New:      "
	conflictDialogLabelExisting = "Existing: "
	conflictDialogLabelSize     = "Size:     "
	conflictDialogLabelModified = "Modified: "
)

func DrawConflictDialog(screen tcell.Screen, layout Layout, state ConflictDialogState, styles theme.Theme, userHomeDir string) {
	if state.Blocker.Kind == jobs.BlockerKindDiskSpace {
		drawConflictDiskSpaceDialog(screen, layout, state, styles, userHomeDir)
		return
	}
	drawConflictFileDialog(screen, layout, state, styles, userHomeDir)
}

func drawConflictFileDialog(screen tcell.Screen, layout Layout, state ConflictDialogState, styles theme.Theme, userHomeDir string) {
	c := state.Blocker.Conflict
	if c == nil {
		c = &jobs.ConflictEvent{}
	}
	width := min(layout.Width-4, 76)
	if width < 48 {
		width = min(48, layout.Width-2)
	}
	// new(3) + blank + existing(3) + sep + prompt + blank + 2 button rows + bottom inner margin
	const height = 14
	rect := draw.CenteredDialogRect(layout, width, height)

	borderStyle := draw.DrawDialogFrame(screen, rect, "File exists", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	body := styles.DialogText.Background(dbg)
	prompt := styles.DialogText.Background(dbg)

	textX := draw.DialogTextX(rect)
	textW := draw.DialogContentWidth(rect)
	y := rect.Y + 1

	y = drawConflictFileGroup(screen, textX, y, textW, conflictDialogLabelNew, c.Source, c.SourceSize, c.SourceTime, body, userHomeDir)
	y++
	y = drawConflictFileGroup(screen, textX, y, textW, conflictDialogLabelExisting, c.Destination, c.DestSize, c.DestTime, body, userHomeDir)

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	primitive.Text(screen, textX, y, textW, "Overwrite this file?", prompt)
	y++
	y++

	row1 := []draw.DialogButtonSpec{
		{Label: "Overwrite", Shortcut: 'O', Focused: state.Focus == 0, Destructive: true},
		{Label: "Skip", Shortcut: 'S', Focused: state.Focus == 1},
		{Label: "Overwrite All", Shortcut: 'A', Focused: state.Focus == 2, Destructive: true},
	}
	row2 := []draw.DialogButtonSpec{
		{Label: "Skip All", Shortcut: 'L', Focused: state.Focus == 3},
		{Label: "Cancel", Shortcut: 'C', Focused: state.Focus == 4},
		{Label: "Postpone", Shortcut: 'P', Focused: state.Focus == 5},
	}
	draw.DrawDialogButtonRowCentered(screen, rect, y, row1, styles)
	y++
	draw.DrawDialogButtonRowCentered(screen, rect, y, row2, styles)
}

func drawConflictDiskSpaceDialog(screen tcell.Screen, layout Layout, state ConflictDialogState, styles theme.Theme, userHomeDir string) {
	d := state.Blocker.DiskSpace
	if d == nil {
		d = &jobs.DiskSpaceBlockerDetails{}
	}
	width := min(layout.Width-4, 76)
	if width < 48 {
		width = min(48, layout.Width-2)
	}
	contentRows := 4 // warn, destination, required, available
	if d.NextSource != "" {
		contentRows++
	}
	// top border + content + separator + prompt + blank + button row + bottom inner margin
	height := 1 + contentRows + 1 + 1 + 1 + 1 + 1
	rect := draw.CenteredDialogRect(layout, width, height)

	borderStyle := draw.DrawDialogFrame(screen, rect, "Disk space", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	body := styles.DialogText.Background(dbg)
	warn := styles.MessageWarn.Background(dbg)

	textX := draw.DialogTextX(rect)
	textW := draw.DialogContentWidth(rect)
	y := rect.Y + 1

	primitive.Text(screen, textX, y, textW, "Not enough free space.", warn)
	y++

	prefixVol := "Destination: "
	lineVol := prefixVol + primitive.FitPathForWidth(primitive.PathWithHomeTilde(d.Destination, userHomeDir), max(1, textW-utf8.RuneCountInString(prefixVol)))
	primitive.Text(screen, textX, y, textW, lineVol, body)
	y++

	primitive.Text(screen, textX, y, textW, "Required:    "+formatConflictDialogBytes(d.RequiredBytes), body)
	y++

	var availLabel string
	if d.AvailableKnown {
		availLabel = "Available:   " + formatConflictDialogBytes(int64(d.AvailableBytes))
	} else {
		availLabel = "Available:   (unknown)"
	}
	primitive.Text(screen, textX, y, textW, availLabel, body)
	y++

	if d.NextSource != "" {
		prefixNext := "Next file:   "
		lineNext := prefixNext + primitive.FitPathForWidth(primitive.PathWithHomeTilde(d.NextSource, userHomeDir), max(1, textW-utf8.RuneCountInString(prefixNext)))
		primitive.Text(screen, textX, y, textW, lineNext, body)
		y++
	}

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	primitive.Text(screen, textX, y, textW, "Make space or retry.", warn)
	y++
	y++

	row := []draw.DialogButtonSpec{
		{Label: "Retry", Shortcut: 'R', Focused: state.Focus == 0},
		{Label: "Abort", Shortcut: 'B', Focused: state.Focus == 1, Destructive: true},
		{Label: "Postpone", Shortcut: 'P', Focused: state.Focus == 2},
	}
	draw.DrawDialogButtonRowCentered(screen, rect, y, row, styles)
}

func drawConflictFileGroup(screen tcell.Screen, textX, y, textW int, pathLabel, path, size, modified string, body tcell.Style, userHomeDir string) int {
	pathAvail := max(1, textW-utf8.RuneCountInString(pathLabel))
	pathLine := pathLabel + primitive.FitPathForWidth(primitive.PathWithHomeTilde(path, userHomeDir), pathAvail)
	primitive.Text(screen, textX, y, textW, pathLine, body)
	y++
	primitive.Text(screen, textX, y, textW, conflictDialogLabelSize+conflictDialogDisplaySize(size), body)
	y++
	primitive.Text(screen, textX, y, textW, conflictDialogLabelModified+conflictDialogDisplayText(modified), body)
	y++
	return y
}

func conflictDialogDisplaySize(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func conflictDialogDisplayText(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func formatConflictDialogBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(n)/1024)
	}
	return fmt.Sprintf("%.1fM", float64(n)/(1024*1024))
}
