package dialog

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// scrollList is a ScrollOnly list of n rows, at most height rows tall, scrolled to offset, with a
// scrollbar (in the spec's ScrollbarStyle). It is windowed: row builds only the rows that fit in
// height, which covers the rows still on screen once dlg.FitSpec shortens the list.
func scrollList(id dlg.ID, n, height, offset int, row func(i int) dlg.ListRow) dlg.Item {
	from := dlg.ScrollViewOffset(offset, height, n)
	to := min(from+max(height, 0), n)
	rows := make([]dlg.ListRow, 0, max(to-from, 0))
	for i := from; i < to; i++ {
		rows = append(rows, row(i))
	}
	return dlg.Item{
		ID: id, Kind: dlg.KindList, ScrollOnly: true, Scrollbar: true, Height: height,
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

// fillList fits spec to layout (dlg.FitSpec) and puts fill(height, width) in place of its list
// id: the list built for the rows and the width it got, for lists whose rows are windowed or
// pre-fitted to the width. The list in spec is a placeholder carrying the height the list wants
// and its MinHeight (which the filled list keeps). ok is false when the dialog does not fit; spec
// is then returned fitted as far as it goes, with the placeholder.
func fillList(spec dlg.Spec, layout Layout, id dlg.ID, fill func(height, width int) dlg.Item) (dlg.Spec, bool) {
	spec, ok := dlg.FitSpec(layout.Width, layout.Height, spec)
	if !ok {
		return spec, false
	}
	p, ok := dlg.Place(layout.Width, layout.Height, spec)
	if !ok {
		return spec, false
	}
	placed, _ := p.Item(id)
	for _, g := range spec.Groups {
		for _, col := range g.Columns {
			for i, it := range col {
				if it.ID == id {
					list := fill(it.Height, placed.Rect.Width)
					list.MinHeight = it.MinHeight
					col[i] = list
				}
			}
		}
	}
	return spec, true
}

// fittedListHeight is the height list id gets in spec fitted to layout, or 0 when the dialog does
// not fit.
func fittedListHeight(spec dlg.Spec, layout Layout, id dlg.ID) int {
	spec, ok := dlg.FitSpec(layout.Width, layout.Height, spec)
	if !ok {
		return 0
	}
	list, _ := spec.Item(id)
	return list.Height
}
