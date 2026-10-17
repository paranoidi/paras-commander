package geom

import "github.com/paranoidi/paras-commander/internal/config"

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
	// SplitVertical stacks primary above secondary (horizontal divider between panes).
	SplitVertical
)

// Layout is the full Phase 1 screen geometry.
type Layout struct {
	Width  int
	Height int
	Menu   Rect
	// StatusCmd is the top-left status_command text area (row 0, before Menu). Zero Rect
	// means the feature is off or ShowMenuBar is false; Menu.X/Width are shrunk accordingly.
	StatusCmd Rect
	Primary   Rect
	Secondary Rect
	Footer    Rect
	// Terminal is the embedded terminal panel strip, directly above Footer. Height is the
	// content row count. Zero Rect means the panel is omitted (not requested, or the
	// terminal was too small to fit alongside the minimum panel area).
	Terminal Rect
	TooSmall bool
}

const (
	// MinFileListContentRows is the default minimum file-list content rows when splitting with a selected strip.
	MinFileListContentRows = 3
	// filePanelListChromeRows is non-list lines in drawPanel (title + column header + bottom frame).
	filePanelListChromeRows = 3
	// selectedStripChromeRows is non-list lines in drawSelectedStrip (title + bottom frame; no column header).
	selectedStripChromeRows = 2
)

const (
	minWidth         = 40
	minHeight        = 8
	minStackedHeight = 16 // menu + footer + two usable pane frames when stacked
	// minTerminalPanelRows is the smallest usable terminal panel content-row count.
	// Requests below this are clamped up; if there isn't room even for this minimum
	// alongside the panel area's own minimum, the terminal panel is omitted.
	minTerminalPanelRows = 3
)

// PanelPaneSplit controls the twin-pane split along the layout axis (width when side-by-side, height when stacked).
// ActivePanel uses the same values as ui.PrimaryPanel (0) and ui.SecondaryPanel (1).
// When Zoom is false, ActivePercent and InactivePercent are ignored (even 50/50 split).
// When Zoom is true, the active pane receives ActivePercent of the axis total and
// the inactive pane receives InactivePercent; both must be positive and sum to 100
// or the layout falls back to an even split.
type PanelPaneSplit struct {
	Zoom              bool
	ActivePanel       int
	ActivePercent     int
	InactivePercent   int
	HideInactivePanel bool
	// SwapPanes draws Secondary in the first visual slot (left/top) and Primary in the second
	// (right/bottom). Identity shares from mainPanelAxisSplit are unchanged.
	SwapPanes bool
}

// PanelWidthSplit is a legacy alias for PanelPaneSplit (split applies to width in side-by-side layout).
type PanelWidthSplit = PanelPaneSplit

// LayoutInput groups terminal size, chrome flags, and split options for CalculateLayout.
type LayoutInput struct {
	Width       int
	Height      int
	ShowMenuBar bool
	Split       PanelPaneSplit
	Orientation SplitOrientation
	// TerminalRows requests an embedded terminal panel with this many content rows
	// (excluding the separator row). Zero omits the panel entirely. See CalculateLayoutWithOrientation
	// for clamping/shrink-to-fit/omission behavior when the terminal panel would starve the panel area.
	TerminalRows int
	// StatusCmdWidth reserves this many columns at the top-left (row 0) for status_command
	// text, shrinking Menu from the left. Zero reserves nothing. Caller clamps this to the
	// configured max width; CalculateLayoutWithOrientation only clamps it to the terminal width.
	StatusCmdWidth int
}

func mainPanelAxisSplit(total int, split PanelPaneSplit) (primaryShare, secondaryShare int) {
	if split.HideInactivePanel {
		if split.ActivePanel == 0 {
			return total, 0
		}
		return 0, total
	}
	primaryShare = total / 2
	secondaryShare = total - primaryShare
	if !split.Zoom {
		return primaryShare, secondaryShare
	}
	ap, ip := split.ActivePercent, split.InactivePercent
	if ap <= 0 || ip <= 0 || ap+ip != 100 {
		return primaryShare, secondaryShare
	}
	primaryPct := ap
	if split.ActivePanel != 0 {
		primaryPct = ip
	}
	primaryShare = (total * primaryPct) / 100
	secondaryShare = total - primaryShare
	const minSplitAxisCells = 10 // cells; if zoomed split is too small, fall back to 50/50
	if primaryShare < minSplitAxisCells || secondaryShare < minSplitAxisCells {
		return total / 2, total - total/2
	}
	return primaryShare, secondaryShare
}

// mainPanelColumnWidths splits total width between primary and secondary columns.
func mainPanelColumnWidths(total int, split PanelPaneSplit) (primaryW, secondaryW int) {
	return mainPanelAxisSplit(total, split)
}

// mainPanelRowHeights splits total height between primary and secondary rows.
func mainPanelRowHeights(total int, split PanelPaneSplit) (primaryH, secondaryH int) {
	return mainPanelAxisSplit(total, split)
}

// MergePaneRects returns one rectangle spanning both browser panes.
// Slot order does not matter (Primary may sit left/top or right/bottom).
func MergePaneRects(primary, secondary Rect, orientation SplitOrientation) Rect {
	x := min(primary.X, secondary.X)
	y := min(primary.Y, secondary.Y)
	if orientation == SplitVertical {
		return Rect{
			X:      x,
			Y:      y,
			Width:  max(primary.Width, secondary.Width),
			Height: primary.Height + secondary.Height,
		}
	}
	return Rect{
		X:      x,
		Y:      y,
		Width:  primary.Width + secondary.Width,
		Height: max(primary.Height, secondary.Height),
	}
}

// CalculateLayout returns deterministic regions for the current terminal size (side-by-side).
func CalculateLayout(width, height int, showMenuBar bool, split PanelPaneSplit) Layout {
	return CalculateLayoutWithOrientation(LayoutInput{
		Width:       width,
		Height:      height,
		ShowMenuBar: showMenuBar,
		Split:       split,
		Orientation: SplitHorizontal,
	})
}

// CalculateLayoutWithOrientation is the orientation-aware layout entry point.
func CalculateLayoutWithOrientation(in LayoutInput) Layout {
	width, height, showMenuBar, split, orientation := in.Width, in.Height, in.ShowMenuBar, in.Split, in.Orientation
	layout := Layout{Width: width, Height: height}
	minH := minHeight
	if orientation == SplitVertical {
		minH = minStackedHeight
	}
	if width < minWidth || height < minH {
		layout.TooSmall = true
		return layout
	}

	menuY := 0
	panelAreaH := height - 1
	if showMenuBar {
		menuY = 1
		panelAreaH = height - 2
		statusW := 0
		if in.StatusCmdWidth > 0 {
			statusW = min(in.StatusCmdWidth, width)
			layout.StatusCmd = Rect{X: 0, Y: 0, Width: statusW, Height: 1}
		}
		layout.Menu = Rect{X: statusW, Y: 0, Width: width - statusW, Height: 1}
	} else {
		layout.Menu = Rect{}
	}
	layout.Footer = Rect{X: 0, Y: height - 1, Width: width, Height: 1}

	if in.TerminalRows > 0 {
		// chromeRows mirrors how panelAreaH was derived above (footer, plus menu when shown),
		// so panelAreaMin is the panel area guaranteed by the TooSmall floor (minH) for this
		// same showMenuBar setting.
		chromeRows := 1
		if showMenuBar {
			chromeRows = 2
		}
		panelAreaMin := minH - chromeRows
		terminalRows := in.TerminalRows
		if terminalRows < minTerminalPanelRows {
			terminalRows = minTerminalPanelRows
		}
		if panelAreaH-terminalRows < panelAreaMin {
			terminalRows = panelAreaH - panelAreaMin
			if terminalRows < minTerminalPanelRows {
				terminalRows = 0
			}
		}
		if terminalRows > 0 {
			layout.Terminal = Rect{X: 0, Y: height - 1 - terminalRows, Width: width, Height: terminalRows}
			panelAreaH -= terminalRows
		}
	}

	if orientation == SplitVertical {
		primaryH, secondaryH := mainPanelRowHeights(panelAreaH, split)
		if split.SwapPanes {
			layout.Secondary = Rect{X: 0, Y: menuY, Width: width, Height: secondaryH}
			layout.Primary = Rect{X: 0, Y: menuY + secondaryH, Width: width, Height: primaryH}
			return layout
		}
		layout.Primary = Rect{X: 0, Y: menuY, Width: width, Height: primaryH}
		layout.Secondary = Rect{X: 0, Y: menuY + primaryH, Width: width, Height: secondaryH}
		return layout
	}

	primaryW, secondaryW := mainPanelColumnWidths(width, split)
	if split.SwapPanes {
		layout.Secondary = Rect{X: 0, Y: menuY, Width: secondaryW, Height: panelAreaH}
		layout.Primary = Rect{X: secondaryW, Y: menuY, Width: primaryW, Height: panelAreaH}
		return layout
	}
	layout.Primary = Rect{X: 0, Y: menuY, Width: primaryW, Height: panelAreaH}
	layout.Secondary = Rect{X: primaryW, Y: menuY, Width: secondaryW, Height: panelAreaH}
	return layout
}

// PanelListRows returns the number of entry rows inside a file panel frame.
func PanelListRows(rect Rect) int {
	if rect.Height < 4 || rect.Width < 8 {
		return 0
	}
	return rect.Height - filePanelListChromeRows
}

// SelectedStripListRows returns list rows inside the selected strip (no Path header row).
func SelectedStripListRows(rect Rect) int {
	if rect.Height < 3 || rect.Width < 8 {
		return 0
	}
	return rect.Height - selectedStripChromeRows
}

// EffectiveSelectedPanelMaxRows returns the configured cap, or the built-in default when n <= 0.
func EffectiveSelectedPanelMaxRows(n int) int {
	if n <= 0 {
		return config.DefaultSelectedPanelMaxRows
	}
	return n
}

const (
	defaultSelectedPanelActivePercent = 50
	selectedPanelActivePercentMin     = 10
	selectedPanelActivePercentMax     = 90
	// minStackedStripFrameW / minStackedFileFrameW gate horizontal strip splits in stacked layout.
	minStackedStripFrameW = 8
	minStackedFileFrameW  = 12
)

// EffectiveSelectedPanelActivePercent returns percent clamped to 10–90, or the default when out of range.
func EffectiveSelectedPanelActivePercent(n int) int {
	if n < selectedPanelActivePercentMin || n > selectedPanelActivePercentMax {
		return defaultSelectedPanelActivePercent
	}
	return n
}

// SelectedStripSplitParams controls how a browser column shares space with the selected strip.
type SelectedStripSplitParams struct {
	StripItemCount     int
	MaxRows            int // unfocused side-by-side cap (0 → default 5)
	ActivePercent      int // focused side-by-side height / stacked width share
	StripFocused       bool
	Orientation        SplitOrientation
	MinFileContentRows int
}

// SplitPanelForSelected divides a browser column into a file list and selected strip.
// Side-by-side: strip under the file list (grows toward ActivePercent of column height when focused).
// Stacked: strip to the right of the file list at ActivePercent of column width, full column height.
// strip.Height == 0 means the strip is omitted.
func SplitPanelForSelected(column Rect, p SelectedStripSplitParams) (file Rect, strip Rect) {
	minFile := p.MinFileContentRows
	if minFile <= 0 {
		minFile = MinFileListContentRows
	}
	if p.Orientation == SplitVertical {
		return splitPanelColumnHorizontal(column, p.StripItemCount, p.ActivePercent, minFile)
	}
	maxRows := p.MaxRows
	if p.StripFocused {
		maxRows = sideBySideFocusedStripContentRows(column.Height, p.ActivePercent, maxRows)
	}
	return SplitPanelColumn(column, p.StripItemCount, maxRows, minFile)
}

// sideBySideFocusedStripContentRows raises the strip content-row cap toward percent of column height.
func sideBySideFocusedStripContentRows(columnHeight, activePercent, maxRows int) int {
	pct := EffectiveSelectedPanelActivePercent(activePercent)
	capRows := columnHeight*pct/100 - selectedStripChromeRows
	if capRows < 1 {
		capRows = 1
	}
	base := EffectiveSelectedPanelMaxRows(maxRows)
	if capRows < base {
		return base
	}
	return capRows
}

// splitPanelColumnHorizontal places the selected strip to the right of the file list (stacked twin panes).
func splitPanelColumnHorizontal(column Rect, stripItemCount, activePercent, minFileContentRows int) (file Rect, strip Rect) {
	minFileFrameH := minFileContentRows + filePanelListChromeRows
	minStripFrameH := selectedStripChromeRows + 1
	if stripItemCount <= 0 ||
		column.Height < minFileFrameH ||
		column.Height < minStripFrameH ||
		column.Width < minStackedFileFrameW+minStackedStripFrameW {
		return column, Rect{X: column.X, Y: column.Y + column.Height, Width: column.Width, Height: 0}
	}
	pct := EffectiveSelectedPanelActivePercent(activePercent)
	stripW := column.Width * pct / 100
	if stripW < minStackedStripFrameW {
		stripW = minStackedStripFrameW
	}
	fileW := column.Width - stripW
	if fileW < minStackedFileFrameW {
		stripW = column.Width - minStackedFileFrameW
		fileW = minStackedFileFrameW
	}
	if stripW < minStackedStripFrameW || fileW < minStackedFileFrameW {
		return column, Rect{X: column.X, Y: column.Y + column.Height, Width: column.Width, Height: 0}
	}
	file = Rect{X: column.X, Y: column.Y, Width: fileW, Height: column.Height}
	strip = Rect{X: column.X + fileW, Y: column.Y, Width: stripW, Height: column.Height}
	return file, strip
}

// SplitPanelColumn divides a column into a top file panel and bottom selected strip.
// stripItemCount is the number of selected paths to show in the strip (0 hides the strip).
// maxStripContentRows caps visible strip rows; extra items scroll within the strip.
// minFileContentRows is the minimum file list content rows when the strip is visible.
// stripRect.Height == 0 means the strip is omitted (caller should not draw it).
func SplitPanelColumn(column Rect, stripItemCount int, maxStripContentRows int, minFileContentRows int) (file Rect, strip Rect) {
	minFileFrameH := minFileContentRows + filePanelListChromeRows
	minStripFrameH := selectedStripChromeRows + 1 // at least one list row
	if stripItemCount <= 0 || column.Height < minFileFrameH+minStripFrameH {
		return column, Rect{X: column.X, Y: column.Y + column.Height, Width: column.Width, Height: 0}
	}
	capRows := EffectiveSelectedPanelMaxRows(maxStripContentRows)
	visibleStrip := stripItemCount
	if visibleStrip > capRows {
		visibleStrip = capRows
	}
	stripFrameH := visibleStrip + selectedStripChromeRows
	fileFrameH := column.Height - stripFrameH

	if fileFrameH < minFileFrameH {
		stripFrameH = column.Height - minFileFrameH
		if stripFrameH < selectedStripChromeRows+1 {
			stripFrameH = selectedStripChromeRows + 1
			fileFrameH = column.Height - stripFrameH
		}
	}

	if stripFrameH < selectedStripChromeRows+1 || fileFrameH < 4 {
		return column, Rect{X: column.X, Y: column.Y + column.Height, Width: column.Width, Height: 0}
	}

	file = Rect{X: column.X, Y: column.Y, Width: column.Width, Height: fileFrameH}
	strip = Rect{X: column.X, Y: column.Y + fileFrameH, Width: column.Width, Height: stripFrameH}
	return file, strip
}

const jobsSubpanelMinFrameH = 4 // Errors/Succeeded panel: title row + bottom border + at least two content rows

// jobsDetailChromeRows is non-content rows in drawJobsDetailPanel (inner area is Height - 2).
const jobsDetailChromeRows = 2

// jobsConflictPanelMinFrameH is the minimum frame height for the file-exists conflict panel above Details.
const jobsConflictPanelMinFrameH = 17

// SplitJobsSecondaryPanels splits the right column into optional conflict (top), detail, optional
// errors, then succeeded. When showConflict is false, conflict has zero height and the rest use the
// full column. errorLineCount is the number of Errors panel text lines; zero hides the panel.
func SplitJobsSecondaryPanels(column Rect, showConflict bool, detailLineCount, errorLineCount int) (conflict, detail, errors, succeeded Rect) {
	if !showConflict {
		d, lower := SplitJobsSecondaryColumn(column, detailLineCount)
		e, su := SplitJobsLowerPanels(lower, errorLineCount)
		return Rect{X: column.X, Y: column.Y, Width: column.Width, Height: 0}, d, e, su
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
	d, lower := SplitJobsSecondaryColumn(sub, detailLineCount)
	e, su := SplitJobsLowerPanels(lower, errorLineCount)
	conflict = Rect{X: column.X, Y: column.Y, Width: column.Width, Height: cH}
	return conflict, d, e, su
}

// SplitJobsLowerPanels divides the area under Details into an Errors panel (top) and a Succeeded
// panel. With errorLineCount == 0 the Errors panel has zero height and Succeeded takes everything.
// Otherwise Errors is capped to its line count plus chrome and to half of the area; an area too
// short for two panels goes entirely to Errors.
func SplitJobsLowerPanels(lower Rect, errorLineCount int) (errors, succeeded Rect) {
	if errorLineCount <= 0 || lower.Height <= 0 {
		return Rect{X: lower.X, Y: lower.Y, Width: lower.Width, Height: 0}, lower
	}
	errH := lower.Height
	if lower.Height >= 2*jobsSubpanelMinFrameH {
		errH = min(errorLineCount+jobsDetailChromeRows, lower.Height/2)
	}
	errors = Rect{X: lower.X, Y: lower.Y, Width: lower.Width, Height: errH}
	succeeded = Rect{X: lower.X, Y: lower.Y + errH, Width: lower.Width, Height: lower.Height - errH}
	return errors, succeeded
}

// SplitJobsSecondaryColumn divides the jobs screen right column into a top Details panel and a lower area
// (Errors + Succeeded, see SplitJobsLowerPanels). The Details frame height is the minimum needed for
// detailLineCount text rows (plus panel chrome), so the lower area receives all remaining vertical space. When
// the column is too short for two usable panels, the lower height is zero and the caller should draw only the
// detail panel in the full column.
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

// JobsPanelContentRows returns scrollable text lines inside a jobs detail/errors/succeeded frame (inner height).
func JobsPanelContentRows(rect Rect) int {
	h := rect.Height - 2
	if h < 0 {
		return 0
	}
	return h
}
