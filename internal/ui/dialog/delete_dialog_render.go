package dialog

import (
	"unicode/utf8"
)

// DeleteDialogMaxListRows caps visible name rows per paint (full list still scrolls).
const DeleteDialogMaxListRows = 18

// listDialogMaxHeight caps the outer height of the delete and copy / move dialogs at 80% of a
// layout of layoutHeight rows (at least 8, but never more than the screen less a row above and
// below), so
// their scrolling lists leave the panels visible around them.
func listDialogMaxHeight(layoutHeight int) int {
	return min(max(layoutHeight*80/100, 8), layoutHeight-2)
}

// deleteListHeight is the delete list's height before the dialog is fitted to the screen: one
// row per entry, up to DeleteDialogMaxListRows.
func deleteListHeight(st FileDialogState) int {
	return max(min(len(st.DeleteEntries), DeleteDialogMaxListRows), 1)
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
	for _, line := range []string{state.DeleteSummary, state.DeleteWarning, state.Note} {
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
