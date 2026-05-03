package ui

// Rect describes a terminal region.
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Layout is the full Phase 1 screen geometry.
type Layout struct {
	Width    int
	Height   int
	Menu     Rect
	Left     Rect
	Right    Rect
	Footer   Rect
	TooSmall bool
}

const (
	defaultSelectionsPanelMaxRows = 5
	minFileListContentRows        = 3
	// filePanelListChromeRows is non-list lines in drawPanel (title + column header + bottom frame).
	filePanelListChromeRows = 3
	// selectionsStripChromeRows is non-list lines in drawSelectionsStrip (title + bottom frame; no column header).
	selectionsStripChromeRows = 2
)

const (
	minWidth  = 40
	minHeight = 8
)

// CalculateLayout returns deterministic regions for the current terminal size.
// When showMenuBar is false, the menu row is omitted and panels extend to the top row.
func CalculateLayout(width, height int, showMenuBar bool) Layout {
	layout := Layout{Width: width, Height: height}
	if width < minWidth || height < minHeight {
		layout.TooSmall = true
		return layout
	}

	leftWidth := width / 2
	rightWidth := width - leftWidth

	if !showMenuBar {
		panelHeight := height - 1
		layout.Menu = Rect{}
		layout.Left = Rect{X: 0, Y: 0, Width: leftWidth, Height: panelHeight}
		layout.Right = Rect{X: leftWidth, Y: 0, Width: rightWidth, Height: panelHeight}
		layout.Footer = Rect{X: 0, Y: height - 1, Width: width, Height: 1}
		return layout
	}

	panelHeight := height - 2
	layout.Menu = Rect{X: 0, Y: 0, Width: width, Height: 1}
	layout.Left = Rect{X: 0, Y: 1, Width: leftWidth, Height: panelHeight}
	layout.Right = Rect{X: leftWidth, Y: 1, Width: rightWidth, Height: panelHeight}
	layout.Footer = Rect{X: 0, Y: height - 1, Width: width, Height: 1}
	return layout
}

// PanelListRows returns the number of entry rows inside a file panel frame.
func PanelListRows(rect Rect) int {
	if rect.Height < 4 || rect.Width < 8 {
		return 0
	}
	return rect.Height - filePanelListChromeRows
}

// SelectionsStripListRows returns list rows inside the selections strip (no Path header row).
func SelectionsStripListRows(rect Rect) int {
	if rect.Height < 3 || rect.Width < 8 {
		return 0
	}
	return rect.Height - selectionsStripChromeRows
}

// EffectiveSelectionsPanelMaxRows returns the configured cap, or the built-in default when n <= 0.
func EffectiveSelectionsPanelMaxRows(n int) int {
	if n <= 0 {
		return defaultSelectionsPanelMaxRows
	}
	return n
}

// SplitPanelColumn divides a column into a top file panel and bottom selections strip.
// stripItemCount is the number of selected paths to show in the strip (0 hides the strip).
// maxStripContentRows caps visible strip rows; extra items scroll within the strip.
// minFileContentRows is the minimum file list content rows when the strip is visible.
// stripRect.Height == 0 means the strip is omitted (caller should not draw it).
func SplitPanelColumn(column Rect, stripItemCount int, maxStripContentRows int, minFileContentRows int) (file Rect, strip Rect) {
	minFileFrameH := minFileContentRows + filePanelListChromeRows
	minStripFrameH := selectionsStripChromeRows + 1 // at least one list row
	if stripItemCount <= 0 || column.Height < minFileFrameH+minStripFrameH {
		return column, Rect{X: column.X, Y: column.Y + column.Height, Width: column.Width, Height: 0}
	}
	capRows := EffectiveSelectionsPanelMaxRows(maxStripContentRows)
	visibleStrip := stripItemCount
	if visibleStrip > capRows {
		visibleStrip = capRows
	}
	stripFrameH := visibleStrip + selectionsStripChromeRows
	fileFrameH := column.Height - stripFrameH

	if fileFrameH < minFileFrameH {
		stripFrameH = column.Height - minFileFrameH
		if stripFrameH < selectionsStripChromeRows+1 {
			stripFrameH = selectionsStripChromeRows + 1
			fileFrameH = column.Height - stripFrameH
		}
	}

	if stripFrameH < selectionsStripChromeRows+1 || fileFrameH < 4 {
		return column, Rect{X: column.X, Y: column.Y + column.Height, Width: column.Width, Height: 0}
	}

	file = Rect{X: column.X, Y: column.Y, Width: column.Width, Height: fileFrameH}
	strip = Rect{X: column.X, Y: column.Y + fileFrameH, Width: column.Width, Height: stripFrameH}
	return file, strip
}

const jobsSubpanelMinFrameH = 4 // activity panel: title row + bottom border + at least two content rows

// jobsDetailChromeRows is non-content rows in drawJobsDetailPanel (inner area is Height - 2).
const jobsDetailChromeRows = 2

// jobsConflictPanelMinFrameH is the minimum frame height for the file-exists conflict panel above Details.
const jobsConflictPanelMinFrameH = 14

// SplitJobsRightPanels splits the right column into optional conflict (top), detail, then activity.
// When showConflict is false, conflict has zero height and detail+activity use the full column.
func SplitJobsRightPanels(column Rect, showConflict bool, detailLineCount int) (conflict, detail, activity Rect) {
	if !showConflict {
		d, a := SplitJobsRightColumn(column, detailLineCount)
		return Rect{X: column.X, Y: column.Y, Width: column.Width, Height: 0}, d, a
	}
	cH := jobsConflictPanelMinFrameH
	if cH >= column.Height {
		cH = max(3, column.Height-1)
	}
	sub := Rect{
		X:      column.X,
		Y:      column.Y + cH,
		Width:  column.Width,
		Height: column.Height - cH,
	}
	d, a := SplitJobsRightColumn(sub, detailLineCount)
	conflict = Rect{X: column.X, Y: column.Y, Width: column.Width, Height: cH}
	return conflict, d, a
}

// SplitJobsRightColumn divides the jobs screen right column into a top Details panel and bottom Activity panel.
// The Details frame height is the minimum needed for detailLineCount text rows (plus panel chrome), so Activity
// receives all remaining vertical space. When the column is too short for two usable panels, activity height is
// zero and the caller should draw only the detail panel in the full column.
func SplitJobsRightColumn(column Rect, detailLineCount int) (detail Rect, activity Rect) {
	activityMin := jobsSubpanelMinFrameH
	minDetailFrame := max(detailLineCount+jobsDetailChromeRows, 3)
	if column.Height < activityMin+3 {
		return column, Rect{X: column.X, Y: column.Y + column.Height, Width: column.Width, Height: 0}
	}
	if minDetailFrame+activityMin <= column.Height {
		detailH := minDetailFrame
		activityH := column.Height - detailH
		detail = Rect{X: column.X, Y: column.Y, Width: column.Width, Height: detailH}
		activity = Rect{X: column.X, Y: column.Y + detailH, Width: column.Width, Height: activityH}
		return detail, activity
	}
	detailH := column.Height - activityMin
	if detailH < 3 {
		return column, Rect{X: column.X, Y: column.Y + column.Height, Width: column.Width, Height: 0}
	}
	detail = Rect{X: column.X, Y: column.Y, Width: column.Width, Height: detailH}
	activity = Rect{X: column.X, Y: column.Y + detailH, Width: column.Width, Height: activityMin}
	return detail, activity
}

// JobsPanelContentRows returns scrollable text lines inside a jobs detail/activity frame (inner height).
func JobsPanelContentRows(rect Rect) int {
	h := rect.Height - 2
	if h < 0 {
		return 0
	}
	return h
}
