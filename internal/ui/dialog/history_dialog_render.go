package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Widget IDs of the History dialog. The path list is driven by the filter input and has no focus
// of its own; an empty state Focus is the filter.
const (
	HistoryFilter dlg.ID = "history-filter"
	HistoryList   dlg.ID = "history-list"
	HistoryZoxide dlg.ID = "history-zoxide"
)

// historyMinWidth is the narrowest terminal-clamped dialog width that still draws.
const historyMinWidth = 36

// historyChromeRows is the outer height of the rows that are not the list: borders, filter,
// separators and the button strip (the zoxide checkbox adds one more).
const historyChromeRows = 6

// HistoryZoxideMnemonic toggles the "Include zoxide" checkbox (Alt+I).
const HistoryZoxideMnemonic = 'I'

// HistoryDialogListRows returns the History dialog's list height for a layout height, or 0
// when the dialog does not fit. zoxideRow adds the "Include zoxide" checkbox row.
func HistoryDialogListRows(layoutHeight int, zoxideRow bool) int {
	chrome := historyChromeRows
	if zoxideRow {
		chrome++
	}
	listH := min(max(layoutHeight-12, 4), 18)
	if chrome+listH > layoutHeight-2 {
		listH = layoutHeight - 2 - chrome
		if listH < 4 {
			return 0
		}
	}
	return listH
}

// historyList is the path list item: the ranked rows with their source icon (own history or
// zoxide), fuzzy highlights, missing-path variant and the pinned / job marks rowMarks resolves.
func historyList(state HistoryDialogState, listRows int, styles theme.Theme, rowMarks RowMarksFunc) dlg.Item {
	list := dlg.Item{
		ID: HistoryList, Kind: dlg.KindList, Driven: true, Fit: dlg.FitMiddle, Scrollbar: true,
		IconOptionFG: true, MarksAfterText: true,
		Cursor: state.Selected, Offset: state.ListScroll, Height: listRows,
	}
	for _, ent := range state.Ranked {
		var row dlg.ListRow
		if ent >= 0 && ent < len(state.DisplayLines) {
			row.Text = state.DisplayLines[ent]
			if ent < len(state.MatchRanges) {
				row.Match = state.MatchRanges[ent]
			}
		}
		if ent >= 0 && ent < len(state.PathMissing) {
			row.Missing = state.PathMissing[ent]
		}
		if ent >= 0 && ent < len(state.Paths) && state.Paths[ent] != "" {
			row.Icon = styles.IconDialogHistory()
			if ent >= state.ZoxideFrom {
				row.Icon = styles.IconDialogZoxide()
			}
			if rowMarks != nil {
				row.Marks = rowMarks(state.Paths[ent])
			}
		}
		list.Rows = append(list.Rows, row)
	}
	return list
}

// EnsureHistoryListScroll keeps Selected in range and visible in the path list of height listRows.
func EnsureHistoryListScroll(state *HistoryDialogState, listRows int) {
	n := len(state.Ranked)
	if n == 0 || listRows <= 0 {
		state.ListScroll = 0
		return
	}
	state.Selected = min(max(state.Selected, 0), n-1)
	state.ListScroll = dlg.Item{Kind: dlg.KindList, Rows: make([]dlg.ListRow, n), Cursor: state.Selected, Offset: state.ListScroll, Height: listRows}.ViewOffset()
}

// HistoryDialogView describes the History dialog: a filter input that steers the path list
// below it, the "Include zoxide" checkbox when zoxide is installed, and OK / Cancel. rowMarks
// may be nil (no pinned / job marks).
func HistoryDialogView(state HistoryDialogState, layout Layout, styles theme.Theme, scrollbarStyle uiscrollbar.Style, rowMarks RowMarksFunc) dlg.Spec {
	listRows := max(HistoryDialogListRows(layout.Height, state.ZoxideAvailable), 4)
	chrome := historyChromeRows
	items := []dlg.Item{{
		ID: HistoryFilter, Kind: dlg.KindInput, Drives: HistoryList, LeadingIcon: styles.IconSearchIcon(),
		Value: state.Query, Cursor: state.QueryCursor, Scroll: state.QueryScroll,
	}}
	if state.ZoxideAvailable {
		chrome++
		items = append(items, dlg.Item{ID: HistoryZoxide, Kind: dlg.KindCheckbox, Label: "Include zoxide", Mnemonic: HistoryZoxideMnemonic, On: state.IncludeZoxide})
	}
	items = append(items, dlg.Item{Kind: dlg.KindSeparator}, historyList(state, listRows, styles, rowMarks))
	title := "History"
	if state.BothPanels {
		title = "History - Both panels"
	}
	return dlg.Spec{
		Title: title, Width: min(78, layout.Width-4), MinWidth: historyMinWidth, MinHeight: chrome + listRows,
		PlainButtonKeys: true, ScrollbarStyle: scrollbarStyle,
		Groups:  []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons: []dlg.Button{{ID: dlg.OK, Label: "OK", Mnemonic: 'O'}, {ID: dlg.Cancel, Label: "Cancel", Mnemonic: 'C'}},
	}
}

func DrawHistoryDialog(screen tcell.Screen, layout Layout, state HistoryDialogState, styles theme.Theme, scrollbarStyle uiscrollbar.Style, rowMarks RowMarksFunc) {
	dlg.Draw(screen, layout.Width, layout.Height, HistoryDialogView(state, layout, styles, scrollbarStyle, rowMarks), state.Focus, styles)
}
