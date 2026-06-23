package geom

// Rect describes a terminal region.
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// SplitOrientation selects how the twin browser panes are arranged.
type SplitOrientation int

const (
	// SplitHorizontal is the default: primary left, secondary right.
	SplitHorizontal SplitOrientation = iota
	// SplitVertical is reserved for a future top/bottom layout (not implemented).
	SplitVertical
)

// Layout is the full Phase 1 screen geometry.
type Layout struct {
	Width     int
	Height    int
	Menu      Rect
	Primary   Rect
	Secondary Rect
	Footer    Rect
	TooSmall  bool
}

const (
	defaultSelectionsPanelMaxRows = 5
	// MinFileListContentRows is the default minimum file-list content rows when splitting with a selections strip.
	MinFileListContentRows = 3
	// filePanelListChromeRows is non-list lines in drawPanel (title + column header + bottom frame).
	filePanelListChromeRows = 3
	// selectionsStripChromeRows is non-list lines in drawSelectionsStrip (title + bottom frame; no column header).
	selectionsStripChromeRows = 2
)

const (
	minWidth  = 40
	minHeight = 8
)

// PanelWidthSplit controls the horizontal split between the two browser columns.
// ActivePanel uses the same values as ui.PrimaryPanel (0) and ui.SecondaryPanel (1).
// When Zoom is false, ActivePercent and InactivePercent are ignored (even 50/50 split).
// When Zoom is true, the active column receives ActivePercent of the total width and
// the inactive column receives InactivePercent; both must be positive and sum to 100
// or the layout falls back to an even split.
type PanelWidthSplit struct {
	Zoom              bool
	ActivePanel       int
	ActivePercent     int
	InactivePercent   int
	HideInactivePanel bool
}

// LayoutInput groups terminal size, chrome flags, and split options for CalculateLayout.
type LayoutInput struct {
	Width       int
	Height      int
	ShowMenuBar bool
	Split       PanelWidthSplit
	Orientation SplitOrientation
}

func mainPanelColumnWidths(total int, split PanelWidthSplit) (primaryW, secondaryW int) {
	if split.HideInactivePanel {
		if split.ActivePanel == 0 {
			return total, 0
		}
		return 0, total
	}
	primaryW = total / 2
	secondaryW = total - primaryW
	if !split.Zoom {
		return primaryW, secondaryW
	}
	ap, ip := split.ActivePercent, split.InactivePercent
	if ap <= 0 || ip <= 0 || ap+ip != 100 {
		return primaryW, secondaryW
	}
	primaryPct := ap
	if split.ActivePanel != 0 {
		primaryPct = ip
	}
	primaryW = (total * primaryPct) / 100
	secondaryW = total - primaryW
	const minSplitColumnWidth = 10 // cells; if zoomed split is too narrow, fall back to 50/50
	if primaryW < minSplitColumnWidth || secondaryW < minSplitColumnWidth {
		return total / 2, total - total/2
	}
	return primaryW, secondaryW
}

// CalculateLayout returns deterministic regions for the current terminal size.
// When showMenuBar is false, the menu row is omitted and panels extend to the top row.
// SplitVertical is not implemented yet: the input is accepted but horizontal math is used today.
func CalculateLayout(width, height int, showMenuBar bool, split PanelWidthSplit) Layout {
	return CalculateLayoutWithOrientation(LayoutInput{
		Width:       width,
		Height:      height,
		ShowMenuBar: showMenuBar,
		Split:       split,
		Orientation: SplitHorizontal,
	})
}

// CalculateLayoutWithOrientation is the orientation-aware layout entry point.
// SplitVertical currently mirrors the horizontal path (future work may swap width/height roles).
func CalculateLayoutWithOrientation(in LayoutInput) Layout {
	width, height, showMenuBar, split := in.Width, in.Height, in.ShowMenuBar, in.Split
	// SplitVertical layout is deferred; horizontal column math is used for all orientations today.
	_ = in.Orientation
	layout := Layout{Width: width, Height: height}
	if width < minWidth || height < minHeight {
		layout.TooSmall = true
		return layout
	}

	primaryWidth, secondaryWidth := mainPanelColumnWidths(width, split)

	if !showMenuBar {
		panelHeight := height - 1
		layout.Menu = Rect{}
		layout.Primary = Rect{X: 0, Y: 0, Width: primaryWidth, Height: panelHeight}
		layout.Secondary = Rect{X: primaryWidth, Y: 0, Width: secondaryWidth, Height: panelHeight}
		layout.Footer = Rect{X: 0, Y: height - 1, Width: width, Height: 1}
		return layout
	}

	panelHeight := height - 2
	layout.Menu = Rect{X: 0, Y: 0, Width: width, Height: 1}
	layout.Primary = Rect{X: 0, Y: 1, Width: primaryWidth, Height: panelHeight}
	layout.Secondary = Rect{X: primaryWidth, Y: 1, Width: secondaryWidth, Height: panelHeight}
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
const jobsConflictPanelMinFrameH = 17

// SplitJobsSecondaryPanels splits the right column into optional conflict (top), detail, then activity.
// When showConflict is false, conflict has zero height and detail+activity use the full column.
func SplitJobsSecondaryPanels(column Rect, showConflict bool, detailLineCount int) (conflict, detail, activity Rect) {
	if !showConflict {
		d, a := SplitJobsSecondaryColumn(column, detailLineCount)
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
	d, a := SplitJobsSecondaryColumn(sub, detailLineCount)
	conflict = Rect{X: column.X, Y: column.Y, Width: column.Width, Height: cH}
	return conflict, d, a
}

// SplitJobsSecondaryColumn divides the jobs screen right column into a top Details panel and bottom Activity panel.
// The Details frame height is the minimum needed for detailLineCount text rows (plus panel chrome), so Activity
// receives all remaining vertical space. When the column is too short for two usable panels, activity height is
// zero and the caller should draw only the detail panel in the full column.
func SplitJobsSecondaryColumn(column Rect, detailLineCount int) (detail Rect, activity Rect) {
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

// SplitJobsSecondaryColumnFlexTop divides a column into a top panel that receives all remaining
// vertical space and a bottom panel sized to bottomLineCount text rows (plus panel chrome).
// When the column is too short for two usable panels, the bottom panel is omitted.
func SplitJobsSecondaryColumnFlexTop(column Rect, bottomLineCount int) (top Rect, bottom Rect) {
	compactMin := jobsSubpanelMinFrameH
	minBottomFrame := max(bottomLineCount+jobsDetailChromeRows, 3)
	if column.Height < compactMin+3 {
		return column, Rect{X: column.X, Y: column.Y + column.Height, Width: column.Width, Height: 0}
	}
	if minBottomFrame+compactMin <= column.Height {
		bottomH := minBottomFrame
		topH := column.Height - bottomH
		top = Rect{X: column.X, Y: column.Y, Width: column.Width, Height: topH}
		bottom = Rect{X: column.X, Y: column.Y + topH, Width: column.Width, Height: bottomH}
		return top, bottom
	}
	bottomH := compactMin
	topH := column.Height - bottomH
	if topH < 3 {
		return column, Rect{X: column.X, Y: column.Y + column.Height, Width: column.Width, Height: 0}
	}
	top = Rect{X: column.X, Y: column.Y, Width: column.Width, Height: topH}
	bottom = Rect{X: column.X, Y: column.Y + topH, Width: column.Width, Height: bottomH}
	return top, bottom
}

// JobsPanelContentRows returns scrollable text lines inside a jobs detail/activity frame (inner height).
func JobsPanelContentRows(rect Rect) int {
	h := rect.Height - 2
	if h < 0 {
		return 0
	}
	return h
}
