package ui

import "github.com/paranoidi/paras-commander/internal/ui/geom"

// Rect and Layout are aliases to shared geometry types used by dialogs and the main UI.
type (
	Rect   = geom.Rect
	Layout = geom.Layout
)

// CalculateLayout returns deterministic regions for the current terminal size.
func CalculateLayout(width, height int, showMenuBar bool, split geom.PanelWidthSplit) Layout {
	return geom.CalculateLayout(width, height, showMenuBar, split)
}

// PanelWidthSplit controls horizontal browser column widths (see geom.PanelWidthSplit).
type PanelWidthSplit = geom.PanelWidthSplit

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

// SplitJobsSecondaryPanels splits the secondary column into optional conflict (top), detail, then activity.
func SplitJobsSecondaryPanels(column Rect, showConflict bool, detailLineCount int) (conflict, detail, activity Rect) {
	return geom.SplitJobsSecondaryPanels(column, showConflict, detailLineCount)
}

// SplitJobsSecondaryColumn divides the jobs screen secondary column into a top Details panel and bottom Activity panel.
func SplitJobsSecondaryColumn(column Rect, detailLineCount int) (detail Rect, activity Rect) {
	return geom.SplitJobsSecondaryColumn(column, detailLineCount)
}

// SplitJobsSecondaryColumnFlexTop divides a column into a flexible top panel and a compact bottom panel.
func SplitJobsSecondaryColumnFlexTop(column Rect, bottomLineCount int) (top Rect, bottom Rect) {
	return geom.SplitJobsSecondaryColumnFlexTop(column, bottomLineCount)
}

// MergeTwinPanelRects returns one rectangle spanning the browser's primary and secondary columns (same height).
func MergeTwinPanelRects(primary, secondary Rect) Rect {
	return Rect{
		X:      primary.X,
		Y:      primary.Y,
		Width:  primary.Width + secondary.Width,
		Height: primary.Height,
	}
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
