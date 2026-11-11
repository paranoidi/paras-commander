package ui

import "github.com/paranoidi/paras-commander/internal/panel"

// SelectedStripLayoutItemCount is the strip row count used for SplitPanelColumn layout
// and matching viewport math. Only the active browser panel shows a selected strip so
// the inactive column does not reserve strip chrome.
func SelectedStripLayoutItemCount(p *panel.State, panelID, activePanel int) int {
	if p == nil {
		return 0
	}
	return SelectedStripLayoutItemCountFromCount(p.SelectedStripCount(), panelID, activePanel)
}

// SelectedStripLayoutItemCountFromCount maps a strip row count to layout chrome without recomputing paths.
func SelectedStripLayoutItemCountFromCount(stripCount, panelID, activePanel int) int {
	if stripCount <= 0 {
		return 0
	}
	if panelID != activePanel {
		return 0
	}
	return stripCount
}
