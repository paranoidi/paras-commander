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
// terminalRows reserves the embedded terminal panel strip (0 = no strip); statusCmdWidth
// reserves the top-left status_command text area (0 = none). Callers must pass the same
// values ui.Render derives from the model so app-side layout math matches what is painted.
func CalculateLayoutWithOrientation(width, height int, showMenuBar bool, split geom.PanelPaneSplit, orientation geom.SplitOrientation, terminalRows int, statusCmdWidth int) Layout {
	return geom.CalculateLayoutWithOrientation(geom.LayoutInput{
		Width:          width,
		Height:         height,
		ShowMenuBar:    showMenuBar,
		Split:          split,
		Orientation:    orientation,
		TerminalRows:   terminalRows,
		StatusCmdWidth: statusCmdWidth,
	})
}

// PanelPaneSplit controls twin-pane split along the layout axis (see geom.PanelPaneSplit).
type PanelPaneSplit = geom.PanelPaneSplit

// PanelWidthSplit is a legacy alias for PanelPaneSplit.
type PanelWidthSplit = geom.PanelPaneSplit

// PanelListTop returns the screen row of the first entry row inside a file panel frame.
func PanelListTop(rect Rect) int {
	return geom.PanelListTop(rect)
}

// PanelListRows returns the number of entry rows inside a file panel frame.
func PanelListRows(rect Rect) int {
	return geom.PanelListRows(rect)
}

// SelectedStripListRows returns list rows inside the selected strip (no Path header row).
func SelectedStripListRows(rect Rect) int {
	return geom.SelectedStripListRows(rect)
}

// EffectiveSelectedPanelMaxRows returns the configured cap, or the built-in default when n <= 0.
func EffectiveSelectedPanelMaxRows(n int) int {
	return geom.EffectiveSelectedPanelMaxRows(n)
}

// EffectiveSelectedPanelActivePercent returns percent clamped to 10–90, or the default when out of range.
func EffectiveSelectedPanelActivePercent(n int) int {
	return geom.EffectiveSelectedPanelActivePercent(n)
}

// SelectedStripSplitParams controls how a browser column shares space with the selected strip.
type SelectedStripSplitParams = geom.SelectedStripSplitParams

// SplitPanelForSelected divides a browser column into a file list and selected strip
// (side-by-side: strip below; stacked: strip to the right).
func SplitPanelForSelected(column Rect, p SelectedStripSplitParams) (file Rect, strip Rect) {
	return geom.SplitPanelForSelected(column, p)
}

// SplitPanelColumn divides a column into a top file panel and bottom selected strip.
func SplitPanelColumn(column Rect, stripItemCount int, maxStripContentRows int, minFileContentRows int) (file Rect, strip Rect) {
	return geom.SplitPanelColumn(column, stripItemCount, maxStripContentRows, minFileContentRows)
}

// SplitJobsSecondaryPanels splits the secondary column into optional conflict (top), detail, then
// operations.
func SplitJobsSecondaryPanels(column Rect, showConflict bool, detailLineCount int) (conflict, detail, operations Rect) {
	return geom.SplitJobsSecondaryPanels(column, showConflict, detailLineCount)
}

// SplitJobsSecondaryColumnFlexTop divides a column into a flexible top panel and a compact bottom panel.
func SplitJobsSecondaryColumnFlexTop(column Rect, bottomLineCount int) (top Rect, bottom Rect) {
	return geom.SplitJobsSecondaryColumnFlexTop(column, bottomLineCount)
}

// MergeTwinPanelRects returns one rectangle spanning the browser's primary and secondary panes.
func MergeTwinPanelRects(primary, secondary Rect, orientation SplitOrientation) Rect {
	return geom.MergePaneRects(primary, secondary, orientation)
}

// JobsPanelContentRows returns scrollable text lines inside a jobs detail/operations frame (inner height).
func JobsPanelContentRows(rect Rect) int {
	return geom.JobsPanelContentRows(rect)
}

// MinFileListContentRows is the default minimum file-list content rows when splitting with a selected strip.
const MinFileListContentRows = geom.MinFileListContentRows

// ScrollOffset returns the scroll start so that selected is centered in the viewport.
func ScrollOffset(selected, visibleRows, total int) int {
	return geom.ScrollOffset(selected, visibleRows, total)
}
