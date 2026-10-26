package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Widget IDs of the Pin dialog. The list is driven by the filter input and has no focus of its own.
const (
	PinFilter dlg.ID = "pin-filter"
	PinList   dlg.ID = "pin-list"
)

// pinMinWidth is the narrowest terminal-clamped dialog width that still draws.
const pinMinWidth = 36

// PinDialogFixedChromeRows is the outer height consumed by rows that aren't the list:
// top border, query row, separator, one blank row above the bottom border, bottom border. No
// OK/Cancel row: Esc closes, the open-in-panel, remove and Enter actions cover every action a
// button row would. The View sizes the dialog with it and internal/app sizes the list with it.
const PinDialogFixedChromeRows = 5

// PinDialogListRows returns the list row count for a dialog sized to layout, mirroring the
// History/PathPicker dialogs' clamp shape (generous outer margin, 4..18 rows). Exported so
// internal/app can size the dialog identically instead of duplicating this computation.
func PinDialogListRows(layoutHeight int) int {
	listH := layoutHeight - 12
	switch {
	case listH > 18:
		listH = 18
	case listH < 4:
		listH = 4
	}
	if PinDialogFixedChromeRows+listH > layoutHeight-2 {
		listH = layoutHeight - 2 - PinDialogFixedChromeRows
		if listH < 4 {
			return 4
		}
	}
	return listH
}

// pinList is the pin list item: the ranked items with their fuzzy highlights, icons and
// missing-path variant. Rows whose item index is out of range stay blank so row indices keep
// matching state.Ranked.
func pinList(state PinDialogState, items []PinDialogItem, listRows int, styles theme.Theme) dlg.Item {
	list := dlg.Item{
		ID: PinList, Kind: dlg.KindList, Driven: true, Fit: dlg.FitPath, Scrollbar: true,
		Cursor: state.Selected, Offset: state.ListScroll, Height: listRows,
	}
	for _, ent := range state.Ranked {
		var row dlg.ListRow
		if ent >= 0 && ent < len(items) {
			it := items[ent]
			row = dlg.ListRow{Text: it.Path, Missing: it.PathMissing, Icon: styles.IconFile()}
			if it.IsDir {
				row.Icon = styles.IconFolder()
			}
			if ent < len(state.MatchRanges) {
				row.Match = state.MatchRanges[ent]
			}
		}
		list.Rows = append(list.Rows, row)
	}
	return list
}

// EnsurePinListScroll keeps Selected in range and visible in the pin list of height listRows.
func EnsurePinListScroll(state *PinDialogState, listRows int) {
	EnsurePickerScroll(len(state.Ranked), &state.Selected, &state.ListScroll, listRows)
}

// PinDialogView describes the Pin dialog: a filter input that steers the pin list below it, a
// blank row and no buttons. The filter always holds focus.
func PinDialogView(state PinDialogState, items []PinDialogItem, layout Layout, styles theme.Theme, scrollbarStyle uiscrollbar.Style) dlg.Spec {
	listRows := PinDialogListRows(layout.Height)
	filter := dlg.Item{
		ID: PinFilter, Kind: dlg.KindInput, Drives: PinList, LeadingIcon: styles.IconSearchIcon(),
		Value: state.Query, Cursor: state.QueryCursor, Scroll: state.QueryScroll,
	}
	return dlg.Spec{
		Title: "Pin", Width: min(78, layout.Width-4), MinWidth: pinMinWidth, MinHeight: PinDialogFixedChromeRows + listRows,
		ScrollbarStyle: scrollbarStyle,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			filter, {Kind: dlg.KindSeparator}, pinList(state, items, listRows, styles), {Kind: dlg.KindLabel},
		}}}},
	}
}

// DrawPinDialog paints the Pin dialog: a fuzzy-filterable, most-recently-pinned-first list of
// pinned files and directories. There are no buttons; Esc closes.
func DrawPinDialog(screen tcell.Screen, layout Layout, state PinDialogState, items []PinDialogItem, styles theme.Theme, scrollbarStyle uiscrollbar.Style) {
	dlg.Draw(screen, layout.Width, layout.Height, PinDialogView(state, items, layout, styles, scrollbarStyle), PinFilter, styles)
}
