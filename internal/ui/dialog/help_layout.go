package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"

// HelpDialogListMetrics holds list geometry for the help dialog (shared by draw and rank sync).
type HelpDialogListMetrics struct {
	Rect       draw.Rect
	InputWidth int
	KeyPad     int
	SecPad     int
	ListH      int
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
	listH := maxH - 9
	if listH < 4 {
		listH = 4
	}
	height := 9 + listH
	if height > layout.Height-2 {
		height = layout.Height - 2
		listH = height - 9
		if listH < 4 {
			return m, false
		}
	}
	rect := draw.CenteredDialogRect(layout, maxW, height)
	leftCol := rect.X + 2
	inputWidth := rect.Width - 4
	if inputWidth < 10 {
		return m, false
	}
	colKey := leftCol
	colSection := leftCol + 28
	if colSection > rect.X+rect.Width-3 {
		colSection = rect.X + rect.Width - 3
	}
	colTitle := leftCol + 50
	if colTitle > rect.X+rect.Width-3 {
		colTitle = rect.X + rect.Width - 3
	}
	m.Rect = rect
	m.InputWidth = inputWidth
	m.KeyPad = colSection - colKey
	m.SecPad = colTitle - colSection
	m.ListH = listH
	return m, true
}
