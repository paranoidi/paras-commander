package dialog

import (
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// HelpDialogChromeRows is the number of fixed (non-list) rows in the help dialog: both
// borders, the filter input, separator-before-list, separator-after-list, and the button
// row. The single source of truth for total-height/list-height math shared by
// HelpDialogChromeRows is the number of fixed (non-list) rows in the help dialog: both
// borders, the filter input, separator-before-list, separator-after-list, and the button
// row. The single source of truth for total-height/list-height math shared by
// ComputeHelpDialogListMetrics and App.helpListRows.
const HelpDialogChromeRows = 6

// HelpDialogListMetrics holds list geometry for the help dialog (shared by draw and rank sync).
type HelpDialogListMetrics struct {
	Rect        draw.Rect
	InputWidth  int
	KeyColWidth int
	ListH       int
}

// helpDialogSize returns the preferred outer width and the visible list row count for the layout;
// ok is false when the dialog does not fit.
func helpDialogSize(layout Layout) (prefW, listH int, ok bool) {
	prefW = min(max(layout.Width-14, 40), 90)
	maxH := min(max(layout.Height-14, 12), 36)
	listH = max(maxH-HelpDialogChromeRows, 4)
	if height := HelpDialogChromeRows + listH; height > layout.Height-2 {
		listH = layout.Height - 2 - HelpDialogChromeRows
		if listH < 4 {
			return 0, 0, false
		}
	}
	return prefW, listH, true
}

// ComputeHelpDialogListMetrics mirrors DrawHelpDialog sizing for the scrollable list area, taking
// the rect from the dialog's placement. When ok is false, the help dialog would not render a
// usable list at this terminal size.
func ComputeHelpDialogListMetrics(layout Layout) (m HelpDialogListMetrics, ok bool) {
	spec, listH, fits := HelpDialogView(HelpViewState{}, layout, theme.Theme{})
	if !fits {
		return m, false
	}
	p, placed := dlg.Place(layout.Width, layout.Height, spec)
	if !placed {
		return m, false
	}
	inputWidth := draw.DialogContentWidth(p.Rect)
	if inputWidth < 10 {
		return m, false
	}
	return HelpDialogListMetrics{Rect: p.Rect, InputWidth: inputWidth, KeyColWidth: min(28, inputWidth-3), ListH: listH}, true
}
