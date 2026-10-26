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

// HistoryZoxideMnemonic toggles the "Include zoxide" checkbox (Alt+I).
const HistoryZoxideMnemonic = 'I'

// HistoryDialogListRows returns the History dialog's list height on layout, or 0 when the
// dialog does not fit.
func HistoryDialogListRows(state HistoryDialogState, layout Layout) int {
	return fittedListHeight(historyFrame(state, layout, theme.Theme{}, ""), layout, HistoryList)
}

// historyList is the path list item: the ranked rows with their source icon (own history or
// zoxide), fuzzy highlights, missing-path variant and the pinned / job marks rowMarks resolves.
func historyList(state HistoryDialogState, listRows int, styles theme.Theme, rowMarks RowMarksFunc) dlg.Item {
	list := dlg.Item{
		ID: HistoryList, Kind: dlg.KindList, Driven: true, Fit: dlg.FitMiddle, Scrollbar: true,
		IconOptionFG: true, MarksAfterText: true,
		Cursor: state.Selected, Offset: state.ListScroll, Height: listRows, RowCount: len(state.Ranked),
	}
	from, to := dlg.ListWindow(state.Selected, state.ListScroll, listRows, len(state.Ranked))
	list.RowsFrom = from
	for _, ent := range state.Ranked[from:to] {
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
	EnsurePickerScroll(len(state.Ranked), &state.Selected, &state.ListScroll, listRows)
}

// HistoryDialogView describes the History dialog: a filter input that steers the path list
// below it, the "Include zoxide" checkbox when zoxide is installed, and OK / Cancel. rowMarks
// may be nil (no pinned / job marks).
func HistoryDialogView(state HistoryDialogState, layout Layout, styles theme.Theme, scrollbarStyle uiscrollbar.Style, rowMarks RowMarksFunc) dlg.Spec {
	spec, _ := fillList(historyFrame(state, layout, styles, scrollbarStyle), layout, HistoryList, func(height, _ int) dlg.Item {
		return historyList(state, height, styles, rowMarks)
	})
	return spec
}

// historyFrame is the History spec with an empty path list as tall as a picker list wants.
func historyFrame(state HistoryDialogState, layout Layout, styles theme.Theme, scrollbarStyle uiscrollbar.Style) dlg.Spec {
	items := []dlg.Item{{
		ID: HistoryFilter, Kind: dlg.KindInput, Drives: HistoryList, LeadingIcon: styles.IconSearchIcon(),
		Value: state.Query, Cursor: state.QueryCursor, Scroll: state.QueryScroll,
	}}
	if state.ZoxideAvailable {
		items = append(items, dlg.Item{ID: HistoryZoxide, Kind: dlg.KindCheckbox, Label: "Include zoxide", Mnemonic: HistoryZoxideMnemonic, On: state.IncludeZoxide})
	}
	items = append(items, dlg.Item{Kind: dlg.KindSeparator},
		dlg.Item{ID: HistoryList, Kind: dlg.KindList, Height: pickerListHeight(layout.Height), MinHeight: pickerMinListRows})
	title := "History"
	if state.BothPanels {
		title = "History - Both panels"
	}
	return dlg.Spec{
		Title: title, Width: min(78, layout.Width-4), MinWidth: historyMinWidth,
		PlainButtonKeys: true, ScrollbarStyle: scrollbarStyle,
		Groups:  []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons: dlg.OKCancelButtons(),
	}
}

func DrawHistoryDialog(screen tcell.Screen, layout Layout, state HistoryDialogState, styles theme.Theme, scrollbarStyle uiscrollbar.Style, rowMarks RowMarksFunc) {
	dlg.Draw(screen, layout.Width, layout.Height, HistoryDialogView(state, layout, styles, scrollbarStyle, rowMarks), state.Focus, styles)
}
