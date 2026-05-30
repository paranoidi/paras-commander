package dialog

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func deleteDialogWarningLines(state FileDialogState) int {
	if strings.TrimSpace(state.DeleteWarning) != "" {
		return 1
	}
	return 0
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

// DeleteDialogListViewportRows returns how many delete-list name rows fit for the terminal height.
func DeleteDialogListViewportRows(layoutHeight int, state FileDialogState) int {
	natural := len(state.DeleteEntries)
	warn := deleteDialogWarningLines(state)
	naturalHeight := 7 + warn + natural
	maxH := deleteDialogMaxHeight(layoutHeight)
	if naturalHeight <= maxH {
		return natural
	}
	available := maxH - 7 - warn
	if available < 1 {
		return 1
	}
	if available > natural {
		return natural
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
	warn := deleteDialogWarningLines(state)
	natural := len(state.DeleteEntries)
	available := outerHeight - 7 - warn
	if available >= natural {
		return natural
	}
	if available < 1 {
		return 1
	}
	return available
}

func fileDeleteDialogHeight(layoutHeight int, state FileDialogState) int {
	warn := deleteDialogWarningLines(state)
	listVP := DeleteDialogListViewportRows(layoutHeight, state)
	height := 7 + warn + listVP
	if height < 8 {
		height = 8
	}
	return height
}

func drawFileDeleteDialogContent(screen tcell.Screen, rect Rect, state FileDialogState, styles theme.Theme, showIcons bool, iconLead int, paintIcon DeleteRowIconPainter) {
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
	bottom := rect.Y + rect.Height - 3
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
	if y >= bottom {
		return
	}
	primitive.Text(screen, rect.X+2, y, innerW, state.DeleteSummary, textStyle)
	y++
	if y >= bottom {
		return
	}
	y++ // blank above list

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
		if y >= bottom {
			break
		}
		if showIcons && paintIcon != nil && iconLead > 0 {
			paintIcon(screen, listCol, y, entry, styles)
		}
		line := DeleteListEntryNameFitsWidth(entry.Name, entry.Path, textW)
		primitive.Text(screen, textX, y, textW, line, listStyle)
		y++
	}
	if y >= bottom {
		return
	}
	y++ // blank below list
	if y >= bottom {
		return
	}
	if warn := strings.TrimSpace(state.DeleteWarning); warn != "" {
		primitive.Text(screen, rect.X+2, y, innerW, warn, textStyle)
	}
}
