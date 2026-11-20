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

// SelectedStripTree reports the session-wide Selected strip tree mode, shared by both file panels
// and the find-duplicates strip. The file panels' strips hold the flag (their row cache reads it);
// the primary panel is the one source the Dedup strip reads.
func (m *Model) SelectedStripTree() bool {
	return m.Primary.Strip.Tree
}

// SetSelectedStripTree switches the Selected strip tree mode for both file panels (and with them
// the Dedup strip).
func (m *Model) SetSelectedStripTree(on bool) {
	m.Primary.Strip.Tree, m.Secondary.Strip.Tree = on, on
}
