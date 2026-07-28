package ui

import "github.com/paranoidi/paras-commander/internal/ui/geom"

// Rect and Layout are aliases to shared geometry types used by dialogs and the main UI.
type (
	Rect             = geom.Rect
	Layout           = geom.Layout
	SplitOrientation = geom.SplitOrientation
)

const (
	SplitHorizontal = geom.SplitHorizontal
	SplitVertical   = geom.SplitVertical
)

// CalculateLayout returns deterministic regions for the current terminal size (side-by-side).
func CalculateLayout(width, height int, showMenuBar bool, split geom.PanelPaneSplit) Layout {
	return geom.CalculateLayout(width, height, showMenuBar, split)
}

// CalculateLayoutWithOrientation is the orientation-aware layout entry point.
// terminalRows reserves the embedded terminal panel strip (0 = no strip); callers must
// pass the same value ui.Render derives from the model so app-side layout math matches
// what is painted.
func CalculateLayoutWithOrientation(width, height int, showMenuBar bool, split geom.PanelPaneSplit, orientation geom.SplitOrientation, terminalRows int) Layout {
	return geom.CalculateLayoutWithOrientation(geom.LayoutInput{
		Width:        width,
		Height:       height,
		ShowMenuBar:  showMenuBar,
		Split:        split,
		Orientation:  orientation,
		TerminalRows: terminalRows,
	})
}

// PanelPaneSplit controls twin-pane split along the layout axis (see geom.PanelPaneSplit).
type PanelPaneSplit = geom.PanelPaneSplit

// PanelWidthSplit is a legacy alias for PanelPaneSplit.
type PanelWidthSplit = geom.PanelPaneSplit

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

// EffectiveSelectionsPanelActivePercent returns percent clamped to 10–90, or the default when out of range.
func EffectiveSelectionsPanelActivePercent(n int) int {
	return geom.EffectiveSelectionsPanelActivePercent(n)
}

// SelectionsStripSplitParams controls how a browser column shares space with the selections strip.
type SelectionsStripSplitParams = geom.SelectionsStripSplitParams

// SplitPanelForSelections divides a browser column into a file list and selections strip
// (side-by-side: strip below; stacked: strip to the right).
func SplitPanelForSelections(column Rect, p SelectionsStripSplitParams) (file Rect, strip Rect) {
	return geom.SplitPanelForSelections(column, p)
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

// MergeTwinPanelRects returns one rectangle spanning the browser's primary and secondary panes.
func MergeTwinPanelRects(primary, secondary Rect, orientation SplitOrientation) Rect {
	return geom.MergePaneRects(primary, secondary, orientation)
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
