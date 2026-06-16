package dialog

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// DeleteDialogMaxListRows caps visible name rows per paint (full list still scrolls).
const DeleteDialogMaxListRows = 18

func deleteDialogWarningLines(state FileDialogState) int {
	if strings.TrimSpace(state.DeleteWarning) != "" {
		return 1
	}
	return 0
}

// deleteDialogChromeRows is interior rows not used by the scrollable name list (footer block + button row).
func deleteDialogChromeRows(state FileDialogState) int {
	return 6 + deleteDialogWarningLines(state)
}

func deleteDialogSummarySepY(rect Rect, state FileDialogState) int {
	return rect.Y + rect.Height - 5 - deleteDialogWarningLines(state)
}

func deleteDialogMaxHeight(layoutHeight int) int {
	maxH := layoutHeight * 80 / 100
	if maxH > layoutHeight-2 {
		maxH = layoutHeight - 2
	}
	if maxH < 8 {
		maxH = 8
	}
	return maxH
}

func deleteDialogCappedEntryCount(natural int) int {
	if natural <= DeleteDialogMaxListRows {
		return natural
	}
	return DeleteDialogMaxListRows
}

// DeleteDialogListViewportRows returns how many delete-list name rows fit for the terminal height.
func DeleteDialogListViewportRows(layoutHeight int, state FileDialogState) int {
	natural := len(state.DeleteEntries)
	capped := deleteDialogCappedEntryCount(natural)
	chrome := deleteDialogChromeRows(state)
	maxH := deleteDialogMaxHeight(layoutHeight)
	available := maxH - chrome
	if available < 1 {
		return 1
	}
	if capped <= available {
		return capped
	}
	return available
}

// DeleteEnsureListScroll clamps DeleteListScroll so the viewport fits.
func DeleteEnsureListScroll(state *FileDialogState, viewportRows, totalRows int) {
	if viewportRows <= 0 || totalRows <= 0 {
		state.DeleteListScroll = 0
		return
	}
	maxScroll := totalRows - viewportRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if state.DeleteListScroll > maxScroll {
		state.DeleteListScroll = maxScroll
	}
	if state.DeleteListScroll < 0 {
		state.DeleteListScroll = 0
	}
}

func deleteDialogListViewportFromHeight(outerHeight int, state FileDialogState) int {
	capped := deleteDialogCappedEntryCount(len(state.DeleteEntries))
	available := outerHeight - deleteDialogChromeRows(state)
	if available >= capped {
		return capped
	}
	if available < 1 {
		return 1
	}
	return available
}

// ComputeDeleteDialogLayoutMinWidth returns the minimum outer dialog width from summary, warning,
// and entry display names. Call once when opening the dialog and store in DeleteLayoutMinWidth.
func ComputeDeleteDialogLayoutMinWidth(state FileDialogState, deleteListIconLead int) int {
	minWidth := 30
	iconLead := deleteListIconLead
	if iconLead < 0 {
		iconLead = 0
	}
	lineWidth := 30
	for _, line := range []string{state.DeleteSummary, state.DeleteWarning} {
		if line == "" {
			continue
		}
		lw := utf8.RuneCountInString(line) + 4
		if lw > lineWidth {
			lineWidth = lw
		}
	}
	for _, entry := range state.DeleteEntries {
		lw := utf8.RuneCountInString(entry.Name) + 4 + iconLead
		if lw > lineWidth {
			lineWidth = lw
		}
	}
	if lineWidth > minWidth {
		minWidth = lineWidth
	}
	return minWidth
}

func fileDeleteDialogHeight(layoutHeight int, state FileDialogState) int {
	listVP := DeleteDialogListViewportRows(layoutHeight, state)
	height := deleteDialogChromeRows(state) + listVP
	if height < 8 {
		height = 8
	}
	return height
}

func drawFileDeleteDialogContent(screen tcell.Screen, rect Rect, state FileDialogState, borderStyle tcell.Style, styles theme.Theme, showIcons bool, iconLead int, paintIcon DeleteRowIconPainter) {
	if state.DeleteSummary == "" && len(state.DeleteEntries) == 0 {
		return
	}
	_, dbg, _ := styles.DialogSurface.Decompose()
	textStyle := styles.DialogText.Background(dbg)
	listStyle := styles.DialogOptionRowStyle(false, false)
	innerW := rect.Width - 4
	if innerW <= 0 {
		return
	}
	listBottom := deleteDialogSummarySepY(rect, state)
	listCol := rect.X + 2
	textX := listCol
	textW := innerW
	if showIcons && paintIcon != nil && iconLead > 0 {
		textX = listCol + iconLead
		textW = innerW - iconLead
		if textW < 1 {
			textW = 1
		}
	}

	y := rect.Y + 1

	vp := deleteDialogListViewportFromHeight(rect.Height, state)
	scroll := state.DeleteListScroll
	maxScroll := len(state.DeleteEntries) - vp
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}
	end := scroll + vp
	if end > len(state.DeleteEntries) {
		end = len(state.DeleteEntries)
	}
	for _, entry := range state.DeleteEntries[scroll:end] {
		if y >= listBottom {
			break
		}
		if showIcons && paintIcon != nil && iconLead > 0 {
			paintIcon(screen, listCol, y, entry, styles)
		}
		line := DeleteListEntryNameFitsWidth(entry.Name, entry.Path, textW)
		primitive.Text(screen, textX, y, textW, line, listStyle)
		y++
	}

	sepY := deleteDialogSummarySepY(rect, state)
	draw.DrawDialogHSeparator(screen, rect, sepY, borderStyle)
	y = sepY + 1
	if warn := strings.TrimSpace(state.DeleteWarning); warn != "" {
		primitive.Text(screen, draw.DialogTextX(rect), y, draw.DialogContentWidth(rect), warn, textStyle)
		y++
	}
	drawDeleteDialogCenteredSummary(screen, rect, y, state.DeleteSummary, textStyle)
}

func drawDeleteDialogCenteredSummary(screen tcell.Screen, rect Rect, y int, summary string, style tcell.Style) {
	if summary == "" {
		return
	}
	innerW := draw.DialogContentWidth(rect)
	x := draw.DialogTextX(rect)
	n := utf8.RuneCountInString(summary)
	if n > innerW {
		primitive.Text(screen, x, y, innerW, summary, style)
		return
	}
	pad := (innerW - n) / 2
	primitive.Text(screen, x+pad, y, innerW-pad, summary, style)
}
