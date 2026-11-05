package dialog

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Icon legend dialog widget IDs.
const (
	IconLegendClose = dlg.ID("close")
	iconLegendList  = dlg.ID("legend")
)

const (
	// iconLegendChrome is the cells the dialog spends beside a row's text (borders, padding, scrollbar).
	iconLegendChrome = 6
	// iconLegendMinWidth keeps the title and the Close button inside the borders.
	iconLegendMinWidth = 24
	// iconLegendMinListH is the fewest rows the list keeps when the screen is short.
	iconLegendMinListH = 4
)

// IconLegendDialogState is the icon legend dialog: one entry per line, scrolled to Scroll.
type IconLegendDialogState struct {
	Open  bool
	Items []primitive.LegendItem
	// Scroll is the first visible row.
	Scroll int
	// Focus is the focused widget ("" = the Close button, the only focus stop).
	Focus dlg.ID
}

// IconLegendDialogView is the legend's widget spec: a ScrollOnly list with one row per entry (the
// icon in its own style, then the label) above a Close button, as tall as the entries unless the
// screen is shorter. ok is false when the dialog does not fit the layout.
func IconLegendDialogView(state IconLegendDialogState, layout Layout, styles theme.Theme) (spec dlg.Spec, ok bool) {
	_, bg, _ := styles.DialogSurface.Decompose()
	w := 0
	for _, it := range state.Items {
		w = max(w, runewidth.StringWidth(it.Icon+" "+it.Label))
	}
	n := len(state.Items)
	spec = dlg.Spec{
		Title: "Icon legend", Width: max(w+iconLegendChrome, iconLegendMinWidth), Unclamped: true, Margin: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			{ID: iconLegendList, Kind: dlg.KindList, Height: n, MinHeight: min(n, iconLegendMinListH)},
		}}}},
		Buttons: []dlg.Button{{ID: IconLegendClose, Label: "Close", Mnemonic: 'C'}},
	}
	return fillList(spec, layout, iconLegendList, func(height, _ int) dlg.Item {
		list := scrollList(iconLegendList, n, height, state.Scroll, func(i int) dlg.ListRow {
			it := state.Items[i]
			return dlg.ListRow{
				Text:  it.Icon + " " + it.Label,
				Spans: []primitive.Span{{Start: 0, End: utf8.RuneCountInString(it.Icon), Style: it.IconStyle(bg)}},
			}
		})
		list.Fit = dlg.FitNone
		return list
	})
}

// DrawIconLegendDialog renders the icon legend modal from its widget spec.
func DrawIconLegendDialog(screen tcell.Screen, layout Layout, state IconLegendDialogState, styles theme.Theme, scrollbarStyle uiscrollbar.Style) {
	if !state.Open {
		return
	}
	if spec, ok := IconLegendDialogView(state, layout, styles); ok {
		spec.ScrollbarStyle = scrollbarStyle
		dlg.Draw(screen, layout.Width, layout.Height, spec, state.Focus, styles)
	}
}
