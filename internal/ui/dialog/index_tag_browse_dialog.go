package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Widget IDs of Browse by tags (the grid is IndexTagsGrid). The buttons are dlg.OK and dlg.Cancel.
const (
	IndexTagBrowseAll       dlg.ID = "index-browse-all"
	IndexTagBrowseAny       dlg.ID = "index-browse-any"
	IndexTagBrowseFlat      dlg.ID = "index-browse-flat"
	IndexTagBrowseStructure dlg.ID = "index-browse-structure"
)

// indexTagBrowseRadioGap is the space between the radios of a row.
const indexTagBrowseRadioGap = 2

// IndexTagBrowseDialogView is the Browse by tags widget spec, fitted to layout: the checkbox grid
// of every tag, the Match radios (All / Any), the View radios (Flat / Structure) and OK / Cancel.
// The jump text shows at the left of the top border. AltOnly holds only while the grid has focus,
// where plain letters are type-to-jump.
func IndexTagBrowseDialogView(st *IndexTagBrowseDialogState, layout Layout, styles theme.Theme) dlg.Spec {
	var items []dlg.Item
	if len(st.Tags) == 0 {
		items = append(items, dlg.Item{Kind: dlg.KindLabel, Label: "No tags yet"})
	} else {
		items = append(items, st.Item())
	}
	radio := func(id dlg.ID, label string, mnemonic rune, on bool) dlg.Item {
		return dlg.Item{ID: id, Kind: dlg.KindRadio, Label: label, Mnemonic: mnemonic, On: on, Gap: indexTagBrowseRadioGap}
	}
	items = append(items,
		dlg.Item{Kind: dlg.KindSeparator},
		dlg.Item{Kind: dlg.KindLabel, Label: "Match:"},
		dlg.Item{Kind: dlg.KindRow, Cells: []dlg.Item{
			radio(IndexTagBrowseAll, "All tags", 'A', !st.Any),
			radio(IndexTagBrowseAny, "Any tag", 'N', st.Any),
		}},
		dlg.Item{Kind: dlg.KindLabel, Label: "View:"},
		dlg.Item{Kind: dlg.KindRow, Cells: []dlg.Item{
			radio(IndexTagBrowseFlat, "Flat list", 'F', !st.Structure),
			radio(IndexTagBrowseStructure, "Directory structure", 'S', st.Structure),
		}},
	)
	spec := dlg.Spec{
		Title: "Browse by tags", TitleQuery: st.JumpQuery, TitleQueryNoMatch: st.jumpNoMatch(),
		Width: st.Width(styles), MinWidth: indexTagsMinWidth,
		MaxHeight: listDialogMaxHeight(layout.Height), Unclamped: true, AltOnly: st.Focus == IndexTagsGrid,
		MarkerWidth: dlg.MarkerWidth(styles),
		Groups:      []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons:     dlg.OKCancelButtons(),
	}
	fitted, _ := dlg.FitSpec(layout.Width, layout.Height, spec)
	return fitted
}

// DrawIndexTagBrowseDialog paints Browse by tags.
func DrawIndexTagBrowseDialog(screen tcell.Screen, layout Layout, st IndexTagBrowseDialogState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, IndexTagBrowseDialogView(&st, layout, styles), st.Focus, styles)
}
