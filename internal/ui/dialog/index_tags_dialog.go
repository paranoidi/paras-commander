package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Widget IDs of the fileindex tag editor (IndexTagsGrid is in index_tag_grid.go).
const (
	IndexTagsInput     dlg.ID = "index-tags-input"
	IndexTagsRecursive dlg.ID = "index-tags-recursive"
)

// IndexTagsDialogView is the tag editor's widget spec, fitted to layout: the checkbox grid of every
// tag, the new-tags input, the Recursive checkbox when a target is a directory and OK / Cancel. The
// jump text shows at the left of the top border. AltOnly holds only while the grid has focus, where
// plain letters are type-to-jump.
func IndexTagsDialogView(st *IndexTagsDialogState, layout Layout, styles theme.Theme) dlg.Spec {
	var items []dlg.Item
	if len(st.Tags) == 0 {
		items = append(items, dlg.Item{Kind: dlg.KindLabel, Label: "No tags yet"})
	} else {
		items = append(items, st.Item())
	}
	items = append(items,
		dlg.Item{Kind: dlg.KindSeparator},
		dlg.Item{Kind: dlg.KindLabel, Label: "New tags (comma or space separated):"},
		dlg.Item{ID: IndexTagsInput, Kind: dlg.KindInput, Value: st.Input, Cursor: st.InputCursor, Scroll: st.InputScroll},
	)
	if st.HasDir {
		items = append(items,
			dlg.Item{Kind: dlg.KindSeparator},
			dlg.Item{ID: IndexTagsRecursive, Kind: dlg.KindCheckbox, Label: "Recursive (whole directory trees)", Mnemonic: 'R', On: st.Recursive},
		)
	}
	spec := dlg.Spec{
		Title: "Tags", TitleQuery: st.JumpQuery, TitleQueryNoMatch: st.jumpNoMatch(),
		Width: st.Width(styles), MinWidth: indexTagsMinWidth,
		MaxHeight: listDialogMaxHeight(layout.Height), Unclamped: true, AltOnly: st.Focus == IndexTagsGrid,
		MarkerWidth: dlg.MarkerWidth(styles),
		Groups:      []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons:     dlg.OKCancelButtons(),
	}
	fitted, _ := dlg.FitSpec(layout.Width, layout.Height, spec)
	return fitted
}

// DrawIndexTagsDialog paints the tag editor.
func DrawIndexTagsDialog(screen tcell.Screen, layout Layout, st IndexTagsDialogState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, IndexTagsDialogView(&st, layout, styles), st.Focus, styles)
}
