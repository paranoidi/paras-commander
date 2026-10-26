package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Widget IDs of the path picker. The list is driven by the filter input and has no focus of its
// own; an empty state Focus is the filter.
const (
	PathPickerFilter dlg.ID = "picker-filter"
	PathPickerList   dlg.ID = "picker-list"
)

const (
	// pathPickerMinWidth is the narrowest terminal-clamped dialog width that still draws.
	pathPickerMinWidth = 36
	// pathPickerChromeRows is the outer height of the rows that are not the list: borders, filter,
	// two separators and the button strip.
	pathPickerChromeRows = 6
)

// PathPickerListRows is how many list rows the path picker shows on a layout of layoutHeight
// rows (4..18; 4 when the dialog does not fit, in which case it is not drawn).
func PathPickerListRows(layoutHeight int) int {
	listH := min(max(layoutHeight-12, 4), 18)
	if pathPickerChromeRows+listH > layoutHeight-2 {
		listH = max(layoutHeight-2-pathPickerChromeRows, 4)
	}
	return listH
}

// pathPickerWidth is the dialog width on a layout of layoutWidth columns.
func pathPickerWidth(layoutWidth int) int { return min(78, layoutWidth-4) }

// pathPickerList is the item list (windowed: only the visible rows are built): each ranked item as
// one pre-fitted row of aligned source,
// name and path columns (the grid is computed over all items, so it stays put while the filter
// changes), its fuzzy highlights mapped onto the columns, the missing-path variant and the
// pinned / job marks rowMarks resolves.
func pathPickerList(state PathPickerState, listRows, rowWidth int, rowMarks RowMarksFunc) dlg.Item {
	marksW := pathPickerRowMarksWidth(state.Items, rowMarks)
	sourceW, nameW, pathW := pathPickerColumnWidths(state.Items, max(rowWidth-marksW, 1))
	list := dlg.Item{
		ID: PathPickerList, Kind: dlg.KindList, Driven: true, Fit: dlg.FitNone, Scrollbar: true,
		MarksWidth: marksW, Cursor: state.Selected, Offset: state.ListScroll, Height: listRows,
		RowCount: len(state.Ranked),
	}
	from, to := dlg.ListWindow(state.Selected, state.ListScroll, listRows, len(state.Ranked))
	list.RowsFrom = from
	for _, ent := range state.Ranked[from:to] {
		var row dlg.ListRow
		if ent >= 0 && ent < len(state.Items) {
			item := state.Items[ent]
			var ranges []search.Range
			if ent < len(state.MatchRanges) {
				ranges = state.MatchRanges[ent]
			}
			text, spans := pathPickerRowContent(item, ranges, sourceW, nameW, pathW, tcell.StyleDefault)
			row = dlg.ListRow{Text: text, Missing: item.PathMissing}
			for _, sp := range spans {
				row.Match = append(row.Match, search.Range{Start: sp.Start, End: sp.End})
			}
			if rowMarks != nil {
				row.Marks = rowMarks(item.Path)
			}
		}
		list.Rows = append(list.Rows, row)
	}
	return list
}

// EnsurePathPickerListScroll keeps Selected in range and visible in the list of height listRows.
func EnsurePathPickerListScroll(state *PathPickerState, listRows int) {
	EnsurePickerScroll(len(state.Ranked), &state.Selected, &state.ListScroll, listRows)
}

// PathPickerView describes the path picker: a filter input (with filesystem completion drawn by
// the caller) that steers the item list below it, and OK / Cancel (Cancel only when the picker
// navigates, since Enter already confirms). rowMarks may be nil (no pinned / job marks).
func PathPickerView(state PathPickerState, layout Layout, styles theme.Theme, scrollbarStyle uiscrollbar.Style, rowMarks RowMarksFunc) dlg.Spec {
	title := state.Title
	if title == "" {
		title = "Choose path"
	}
	width := pathPickerWidth(layout.Width)
	listRows := PathPickerListRows(layout.Height)
	filter := dlg.Item{
		ID: PathPickerFilter, Kind: dlg.KindInput, Drives: PathPickerList, LeadingIcon: styles.IconSearchIcon(),
		Value: state.Query, Cursor: state.QueryCursor, Scroll: state.QueryScroll,
		CompletionSuffix: state.Completion.GhostSuffix(state.Query),
		Invalid:          state.QueryPathInvalid && !state.QueryPathCheckPending,
	}
	buttons := dlg.OKCancelButtons()
	if state.Purpose == PathPickerPurposeNavigate {
		buttons = buttons[1:]
	}
	return dlg.Spec{
		Title: title, Width: width, MinWidth: pathPickerMinWidth, MinHeight: pathPickerChromeRows + listRows,
		PlainButtonKeys: true, ScrollbarStyle: scrollbarStyle,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			filter, {Kind: dlg.KindSeparator}, pathPickerList(state, listRows, width-4, rowMarks),
		}}}},
		Buttons: buttons,
	}
}

func DrawPathPickerDialog(screen tcell.Screen, layout Layout, state PathPickerState, styles theme.Theme, scrollbarStyle uiscrollbar.Style, rowMarks RowMarksFunc) {
	spec := PathPickerView(state, layout, styles, scrollbarStyle, rowMarks)
	p, ok := dlg.Draw(screen, layout.Width, layout.Height, spec, state.Focus, styles)
	if !ok {
		return
	}
	// The completion dropdown is an overlay anchored to the filter input, painted over the list.
	if in, ok := p.Item(PathPickerFilter); ok {
		drawPathCompletionDropdown(screen, in.Rect.X+draw.LeadingIconCols(styles.IconSearchIcon()), in.Rect.Y+1, state.QueryScroll, state.Completion, scrollbarStyle, styles)
	}
}
