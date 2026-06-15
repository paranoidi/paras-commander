package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// FindDialogListRows returns the visible find-dialog list row count (matches DrawFindDialog).
func FindDialogListRows(layout Layout, showSearchSelectionsOption bool) int {
	_, _, listH, ok := dialog.FindDialogMetrics(layout, showSearchSelectionsOption)
	if !ok {
		return 4
	}
	return listH
}

// FindDialogSelectionSizePadded returns the padded selection count/size label for the find dialog separator, or "".
func FindDialogSelectionSizePadded(
	state *FindDialogState,
	painter DiskUsagePainter,
	descendIntoMountPoints bool,
	goduIgnore func(string) bool,
	workingSym string,
) string {
	if state == nil {
		return ""
	}
	raw, ok := state.MarkedSelectionSizeLabel(
		false,
		painter,
		descendIntoMountPoints,
		goduIgnore,
		workingSym,
	)
	if !ok {
		return ""
	}
	return SelectionSizePadded(raw)
}

// PaintFindDialog repaints the find dialog overlay without touching panels or the footer.
func PaintFindDialog(
	screen tcell.Screen,
	layout Layout,
	state *FindDialogState,
	styles theme.Theme,
	showIcons bool,
	painter DiskUsagePainter,
	descendIntoMountPoints bool,
	goduIgnore func(string) bool,
) {
	if state == nil || !state.Open {
		return
	}
	selectionLabel := FindDialogSelectionSizePadded(state, painter, descendIntoMountPoints, goduIgnore, styles.SymbolWorking())
	dialog.DrawFindDialog(screen, layout, *state, styles, showIcons, DialogListIconLeadingWidth(showIcons), PaintFindDialogRowIcon, selectionLabel)
}
