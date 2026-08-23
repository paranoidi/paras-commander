package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"

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

// ComputeHelpDialogListMetrics mirrors DrawHelpDialog sizing for the scrollable list area.
// When ok is false, the help dialog would not render a usable list at this terminal size.
func ComputeHelpDialogListMetrics(layout Layout) (m HelpDialogListMetrics, ok bool) {
	maxW := layout.Width - 14
	if maxW < 40 {
		maxW = 40
	}
	if maxW > 90 {
		maxW = 90
	}
	maxH := layout.Height - 14
	if maxH < 12 {
		maxH = 12
	}
	if maxH > 36 {
		maxH = 36
	}
	listH := maxH - HelpDialogChromeRows
	if listH < 4 {
		listH = 4
	}
	height := HelpDialogChromeRows + listH
	if height > layout.Height-2 {
		height = layout.Height - 2
		listH = height - HelpDialogChromeRows
		if listH < 4 {
			return m, false
		}
	}
	rect := draw.CenteredDialogRect(layout, maxW, height)
	inputWidth := rect.Width - 4
	if inputWidth < 10 {
		return m, false
	}
	keyColWidth := 28
	if keyColWidth > inputWidth-3 {
		keyColWidth = inputWidth - 3
	}
	m.Rect = rect
	m.InputWidth = inputWidth
	m.KeyColWidth = keyColWidth
	m.ListH = listH
	return m, true
}
