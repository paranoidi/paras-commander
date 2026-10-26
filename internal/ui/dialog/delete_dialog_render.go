package dialog

import (
	"strings"
	"unicode/utf8"
)

// DeleteDialogMaxListRows caps visible name rows per paint (full list still scrolls).
const DeleteDialogMaxListRows = 18

func deleteDialogWarningLines(state FileDialogState) int {
	if strings.TrimSpace(state.DeleteWarning) != "" {
		return 1
	}
	return 0
}

// deleteDialogChromeRows is interior rows not used by the scrollable name list
// (footer block + blank + button row).
func deleteDialogChromeRows(state FileDialogState) int {
	return 7 + deleteDialogWarningLines(state)
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
