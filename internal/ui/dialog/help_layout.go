package dialog

import (
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// helpMinListRows is the fewest entry rows the help dialog shows; below that it is not drawn.
const helpMinListRows = 4

// HelpDialogListMetrics holds list geometry for the help dialog (shared by draw and rank sync).
type HelpDialogListMetrics struct {
	Rect        draw.Rect
	InputWidth  int
	KeyColWidth int
	ListH       int
}

// helpDialogSize returns the help dialog's preferred outer width and its outer height for the
// layout: a margin around it and between 12 and 36 rows. The entry list takes the rows the filter
// and the button strip leave.
func helpDialogSize(layout Layout) (prefW, height int) {
	return min(max(layout.Width-14, 40), 90), min(max(layout.Height-14, 12), 36)
}

// ComputeHelpDialogListMetrics is the help dialog's list geometry, from the dialog's placement: the
// widths FormatHelpRow formats entries to and the visible rows. When ok is false, the help dialog would not render a
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
