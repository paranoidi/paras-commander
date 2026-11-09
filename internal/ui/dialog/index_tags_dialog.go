package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Widget IDs of the fileindex tag editor.
const (
	IndexTagsInput     dlg.ID = "index-tags-input"
	IndexTagsGrid      dlg.ID = "index-tags-grid"
	IndexTagsRecursive dlg.ID = "index-tags-recursive"
)

const (
	indexTagsMinWidth     = 46
	indexTagsMinListRows  = 3
	indexTagsWidthPadding = 8 // borders, margins and the gap between the columns
)

// IndexTagsDialogView is the tag editor's widget spec, fitted to layout: the checkbox grid of every
// tag, a separator that shows the jump text, the new-tags input, the Recursive checkbox and
// OK / Cancel. AltOnly holds only while the grid has focus, where plain letters are type-to-jump.
func IndexTagsDialogView(st *IndexTagsDialogState, layout Layout, styles theme.Theme) dlg.Spec {
	var items []dlg.Item
	if len(st.Tags) == 0 {
		items = append(items, dlg.Item{Kind: dlg.KindLabel, Label: "No tags yet"})
	} else {
		items = append(items, indexTagsGrid(st))
	}
	items = append(items,
		dlg.Item{Kind: dlg.KindSeparator, Label: st.jumpLabel()},
		dlg.Item{Kind: dlg.KindLabel, Label: "New tags (comma or space separated):"},
		dlg.Item{ID: IndexTagsInput, Kind: dlg.KindInput, Value: st.Input, Cursor: st.InputCursor, Scroll: st.InputScroll},
		dlg.Item{Kind: dlg.KindSeparator},
		dlg.Item{ID: IndexTagsRecursive, Kind: dlg.KindCheckbox, Label: "Recursive (whole directory trees)", Mnemonic: 'R', On: st.Recursive},
	)
	spec := dlg.Spec{
		Title: "Tags", Width: indexTagsWidth(st, styles), MinWidth: indexTagsMinWidth,
		MaxHeight: listDialogMaxHeight(layout.Height), Unclamped: true, AltOnly: st.Focus == IndexTagsGrid,
		MarkerWidth: dlg.MarkerWidth(styles),
		Groups:      []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons:     dlg.OKCancelButtons(),
	}
	fitted, _ := dlg.FitSpec(layout.Width, layout.Height, spec)
	return fitted
}

// indexTagsWidth is the dialog width that shows the longest tag in both columns.
func indexTagsWidth(st *IndexTagsDialogState, styles theme.Theme) int {
	longest := 0
	for _, t := range st.Tags {
		longest = max(longest, runewidth.StringWidth(t.Name))
	}
	return max(indexTagsColumns*(longest+dlg.MarkerWidth(styles))+indexTagsWidthPadding, indexTagsMinWidth)
}

// indexTagsGrid is the checkbox grid; the cells matching the jump text show the matched letters.
func indexTagsGrid(st *IndexTagsDialogState) dlg.Item {
	rows := (len(st.Tags) + indexTagsColumns - 1) / indexTagsColumns
	grid := dlg.Item{
		ID: IndexTagsGrid, Kind: dlg.KindList, Markers: true, CellColumns: indexTagsColumns, Scrollbar: true,
		Cursor: st.Cursor, Col: st.Col, Offset: st.Offset, Height: rows, MinHeight: min(indexTagsMinListRows, rows),
		Rows: make([]dlg.ListRow, rows),
	}
	for i, t := range st.Tags {
		cell := dlg.ListCell{Text: t.Name, Check: t.State}
		if st.JumpQuery != "" {
			cell.Match = search.Fuzzy(st.JumpQuery, t.Name, search.Options{CaseInsensitive: true}).Ranges
		}
		row := &grid.Rows[i/indexTagsColumns]
		row.Cells = append(row.Cells, cell)
	}
	return grid
}

// DrawIndexTagsDialog paints the tag editor.
func DrawIndexTagsDialog(screen tcell.Screen, layout Layout, st IndexTagsDialogState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, IndexTagsDialogView(&st, layout, styles), st.Focus, styles)
}
