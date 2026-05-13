package ui

import "github.com/paranoidi/paras-commander/internal/ui/geom"

// Rect and Layout are aliases to shared geometry types used by dialogs and the main UI.
type (
	Rect   = geom.Rect
	Layout = geom.Layout
)

// CalculateLayout returns deterministic regions for the current terminal size.
func CalculateLayout(width, height int, showMenuBar bool) Layout {
	return geom.CalculateLayout(width, height, showMenuBar)
}

// PanelListRows returns the number of entry rows inside a file panel frame.
func PanelListRows(rect Rect) int {
	return geom.PanelListRows(rect)
}

// SelectionsStripListRows returns list rows inside the selections strip (no Path header row).
func SelectionsStripListRows(rect Rect) int {
	return geom.SelectionsStripListRows(rect)
}

// EffectiveSelectionsPanelMaxRows returns the configured cap, or the built-in default when n <= 0.
func EffectiveSelectionsPanelMaxRows(n int) int {
	return geom.EffectiveSelectionsPanelMaxRows(n)
}

// SplitPanelColumn divides a column into a top file panel and bottom selections strip.
func SplitPanelColumn(column Rect, stripItemCount int, maxStripContentRows int, minFileContentRows int) (file Rect, strip Rect) {
	return geom.SplitPanelColumn(column, stripItemCount, maxStripContentRows, minFileContentRows)
}

// SplitJobsRightPanels splits the right column into optional conflict (top), detail, then activity.
func SplitJobsRightPanels(column Rect, showConflict bool, detailLineCount int) (conflict, detail, activity Rect) {
	return geom.SplitJobsRightPanels(column, showConflict, detailLineCount)
}

// SplitJobsRightColumn divides the jobs screen right column into a top Details panel and bottom Activity panel.
func SplitJobsRightColumn(column Rect, detailLineCount int) (detail Rect, activity Rect) {
	return geom.SplitJobsRightColumn(column, detailLineCount)
}

// JobsPanelContentRows returns scrollable text lines inside a jobs detail/activity frame (inner height).
func JobsPanelContentRows(rect Rect) int {
	return geom.JobsPanelContentRows(rect)
}

// MinFileListContentRows is the default minimum file-list content rows when splitting with a selections strip.
const MinFileListContentRows = geom.MinFileListContentRows

// ScrollOffset returns the scroll start so that selected is centered in the viewport.
func ScrollOffset(selected, visibleRows, total int) int {
	return geom.ScrollOffset(selected, visibleRows, total)
}
