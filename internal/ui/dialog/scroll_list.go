package dialog

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// scrollList is a ScrollOnly list of n rows, height rows tall, scrolled to offset, with a
// scrollbar (in the spec's ScrollbarStyle). It is windowed: row builds only the rows on screen. PgUp/PgDn move a full page.
func scrollList(id dlg.ID, n, height, offset int, row func(i int) dlg.ListRow) dlg.Item {
	from := dlg.ScrollViewOffset(offset, height, n)
	to := min(from+max(height, 0), n)
	rows := make([]dlg.ListRow, 0, max(to-from, 0))
	for i := from; i < to; i++ {
		rows = append(rows, row(i))
	}
	return dlg.Item{
		ID: id, Kind: dlg.KindList, ScrollOnly: true, Scrollbar: true, Height: height, PageStep: height,
		Offset: offset, RowCount: n, RowsFrom: from, Rows: rows,
	}
}

// listIconPad is the ListRow.Icon that keeps a row's leading iconLead cells free for a devicon
// painted after dlg.Draw (paintListIcons); "" without icons.
func listIconPad(iconLead int) string {
	if iconLead <= 0 {
		return ""
	}
	return strings.Repeat(" ", iconLead-1)
}

// paintListIcons calls paint for every drawn row of list id with the cell its devicon goes in:
// the cells listIconPad kept free at the start of the row.
func paintListIcons(p dlg.Placement, id dlg.ID, paint func(row, x, y int)) {
	list, _ := p.Item(id)
	p.EachListRow(id, func(row, y int) { paint(row, list.Rect.X, y) })
}
