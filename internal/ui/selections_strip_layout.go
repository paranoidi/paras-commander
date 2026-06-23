package ui

import "github.com/paranoidi/paras-commander/internal/panel"

// SelectionsStripLayoutItemCount is the strip row count used for SplitPanelColumn layout
// and matching viewport math. Only the active browser panel shows a selections strip so
// the inactive column does not reserve strip chrome. When the theme picker is open, the
// left column always includes its strip (if any) so the preview matches the styled left panel.
func SelectionsStripLayoutItemCount(p *panel.State, panelID, activePanel int, themePickerOpen bool) int {
	if p == nil {
		return 0
	}
	return SelectionsStripLayoutItemCountFromCount(p.SelectionsStripCount(), panelID, activePanel, themePickerOpen)
}

// SelectionsStripLayoutItemCountFromCount maps a strip row count to layout chrome without recomputing paths.
func SelectionsStripLayoutItemCountFromCount(stripCount, panelID, activePanel int, themePickerOpen bool) int {
	if stripCount <= 0 {
		return 0
	}
	if themePickerOpen && panelID == PrimaryPanel {
		return stripCount
	}
	if panelID != activePanel {
		return 0
	}
	return stripCount
}
