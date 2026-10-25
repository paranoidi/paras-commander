package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// FindDialogListRows returns the visible find-dialog list row count (matches DrawFindDialog).
func FindDialogListRows(layout Layout, showSearchSelectionsOption bool) int {
	_, _, listH, ok := dialog.FindDialogMetrics(layout, showSearchSelectionsOption)
	if !ok {
		return 4
	}
	return listH
}

// FindDialogSelectionSizeLabel returns the unpadded selection count/size label for the find
// dialog's button separator (the dialog engine pads it), or "".
func FindDialogSelectionSizeLabel(
	state *dialog.FindDialogState,
	painter DiskUsagePainter,
	workingSym string,
) string {
	if state == nil {
		return ""
	}
	raw, ok := state.MarkedSelectionSizeLabel(false, painter, workingSym)
	if !ok {
		return ""
	}
	return raw
}

// PaintFindDialog repaints the find dialog overlay without touching panels or the footer.
func PaintFindDialog(
	screen tcell.Screen,
	layout Layout,
	state *dialog.FindDialogState,
	styles theme.Theme,
	showIcons bool,
	scrollbarStyle uiscrollbar.Style,
	painter DiskUsagePainter,
	pinnedItems []PinnedItem,
	jobMarks []JobPathMark,
) {
	if state == nil || !state.Open {
		return
	}
	selectionLabel := FindDialogSelectionSizeLabel(state, painter, styles.IconWorking())
	ctx := dialog.DialogRenderContext{
		Styles:         styles,
		ShowIcons:      showIcons,
		IconLead:       DialogListIconLeadingWidth(showIcons),
		ScrollbarStyle: scrollbarStyle,
	}
	dialog.DrawFindDialog(screen, layout, *state, ctx, PaintFindDialogRowIcon, selectionLabel, rowMarksResolver(pinnedItems, jobMarks))
}
