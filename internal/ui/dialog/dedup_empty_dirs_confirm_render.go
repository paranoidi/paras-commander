package dialog

import (
	"fmt"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"

	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// dedupEmptyDirsConfirmMaxListRows caps visible directory rows (a trailing
// "(+N more)" line takes the last slot when the list is longer).
const dedupEmptyDirsConfirmMaxListRows = 10

// DedupEmptyDirsRowIconPainter draws a folder devicon for one row of the
// empty-dirs confirmation list; nil skips icons. dir is the relative path
// shown on that row (used only to derive the display name for lookup).
type DedupEmptyDirsRowIconPainter func(screen tcell.Screen, x, y int, dir string, styles theme.Theme)

func dedupEmptyDirsConfirmListRows(state DedupEmptyDirsConfirmState) int {
	if len(state.Dirs) <= dedupEmptyDirsConfirmMaxListRows {
		return max(len(state.Dirs), 1)
	}
	return dedupEmptyDirsConfirmMaxListRows
}

func dedupEmptyDirsConfirmWidth(state DedupEmptyDirsConfirmState, msg string, iconLead int) int {
	width := utf8.RuneCountInString(msg) + 4
	for _, dir := range state.Dirs {
		if w := utf8.RuneCountInString(dir) + 4 + iconLead; w > width {
			width = w
		}
	}
	if width < 50 {
		width = 50
	}
	if width > 78 {
		width = 78
	}
	return width
}

func DrawDedupEmptyDirsConfirmDialog(screen tcell.Screen, layout Layout, state DedupEmptyDirsConfirmState, styles theme.Theme, showIcons bool, iconLead int, paintIcon DedupEmptyDirsRowIconPainter) {
	noun := "directories"
	if len(state.Dirs) == 1 {
		noun = "directory"
	}
	msg := fmt.Sprintf("Remove %d empty %s left by this delete?", len(state.Dirs), noun)

	listRows := dedupEmptyDirsConfirmListRows(state)
	width := dedupEmptyDirsConfirmWidth(state, msg, iconLead)
	height := listRows + 7 // msg + blank + list + separator + blank + buttons + top/bottom border
	rect := draw.CenteredDialogRect(layout, width, height)

	borderStyle := draw.DrawDialogFrame(screen, rect, "Remove Empty Directories", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	textStyle := styles.DialogText.Background(dbg)
	listCol := draw.DialogTextX(rect)
	innerW := draw.DialogContentWidth(rect)
	textX, textW := listCol, innerW
	if showIcons && paintIcon != nil && iconLead > 0 {
		textX = listCol + iconLead
		textW = max(innerW-iconLead, 1)
	}

	primitive.Text(screen, listCol, rect.Y+1, innerW, msg, textStyle)

	listY := rect.Y + 3
	shown := len(state.Dirs)
	truncated := shown > dedupEmptyDirsConfirmMaxListRows
	if truncated {
		shown = dedupEmptyDirsConfirmMaxListRows - 1
	}
	for i := 0; i < shown; i++ {
		if showIcons && paintIcon != nil && iconLead > 0 {
			paintIcon(screen, listCol, listY+i, state.Dirs[i], styles)
		}
		primitive.Text(screen, textX, listY+i, textW, primitive.FitPathForWidth(state.Dirs[i], textW), textStyle)
	}
	if truncated {
		more := fmt.Sprintf("(+%d more)", len(state.Dirs)-shown)
		primitive.Text(screen, textX, listY+shown, textW, more, textStyle)
	}

	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogHSeparator(screen, rect, buttonY-2, borderStyle)
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "Yes", Shortcut: 'Y', Focused: state.Focus == 0},
		{Label: "No", Shortcut: 'N', Focused: state.Focus == 1},
	}, styles)
}
