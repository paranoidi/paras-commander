package ui

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/quickfilter"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// jobsDetailLineBudgetFallback is used when line width is unknown (layout line counts only).
const jobsDetailLineBudgetFallback = 4096

// Jobs list column layout to the Status column: icon, type (widest jobs.Type is "delete"), separator space.
const (
	jobsListColIcon      = 3
	jobsListColTypeRunes = 7 // internal/jobs.Type: extract (longest)
	jobsListColTypeCell  = jobsListColTypeRunes + 1
	jobsListColPrefix    = jobsListColIcon + jobsListColTypeCell // offset from row start to Status column
)
const jobsListColStatus = 10
const jobsListColETA = 10
const jobsListColSpeed = 10

// ViewMode selects between the twin file browser and auxiliary full-screen views.
type ViewMode int

const (
	ViewBrowser ViewMode = iota
	ViewJobs
	ViewCommands
	ViewCompare
	ViewMessages
	ViewFilePreview
	ViewDedup
)

// IsAuxiliaryView reports vm is a full-screen meta view rather than the file browser.
func IsAuxiliaryView(vm ViewMode) bool {
	return vm == ViewJobs || vm == ViewCommands || vm == ViewCompare || vm == ViewMessages || vm == ViewFilePreview || vm == ViewDedup
}

// PanelZoomSplitsColumns reports whether the browser uses an asymmetric column split
// (Model.PanelZoomEnabled, from [ui.zoom].active_panel plus session override, preview suppression, and width gate).
// It applies only to the twin file browser; jobs/commands/messages use an even split.
func PanelZoomSplitsColumns(vm ViewMode, zoomEnabled bool) bool {
	return zoomEnabled && !IsAuxiliaryView(vm)
}

// LayoutHideInactivePanel reports whether layout should give the active browser column
// full width and omit the inactive file panel. Model.HideInactivePanel is retained across
// view switches but affects layout only in ViewBrowser.
func LayoutHideInactivePanel(vm ViewMode, hideInactivePanel bool) bool {
	return hideInactivePanel && vm == ViewBrowser
}

// JobsViewState holds focus and scroll positions for the jobs view screen.
type JobsViewState struct {
	Selected        int
	FocusPane    int // index into JobsPaneOrder: list, blocker (if shown), Details, Operations (if shown)
	ListScroll   int
	DetailScroll int
	OpsScroll    int
	// OpsResultHidden is the set of results the Operations pane hides (zero shows all; see
	// OpsResultShown). OpsFilter holds the Operations fuzzy query; only its Query, Cursor and
	// Editing are used (VisibleJobOperations matches rows itself).
	OpsResultHidden dialog.JobResultFilter
	OpsFilter       quickfilter.Filter
	// ConflictButtonFocus is the focused action index in the conflict panel (0..5).
	ConflictButtonFocus int
	// Maximized draws the focused Operations pane fullscreen with wrapped lines.
	Maximized bool
}

// OpsResultShown is the result filter of the Operations pane: the results it shows.
func (s JobsViewState) OpsResultShown() dialog.JobResultFilter {
	return dialog.JobResultAll &^ s.OpsResultHidden
}

// JobsMaximizedRect is the bounding rect of the jobs Primary and Secondary columns, where a
// maximized Operations pane is drawn.
func JobsMaximizedRect(layout Layout) Rect {
	p, s := layout.Primary, layout.Secondary
	x, y := min(p.X, s.X), min(p.Y, s.Y)
	return Rect{X: x, Y: y, Width: max(p.X+p.Width, s.X+s.Width) - x, Height: max(p.Y+p.Height, s.Y+s.Height) - y}
}

// jobsMaximizedPane returns the pane drawn full screen over the whole jobs view: the focused
// Operations pane while maximized. ok is false when the normal split is drawn.
func jobsMaximizedPane(layout Layout, state JobsViewState, sel JobEntry, now time.Time, throughputChartEnabled bool) (pane JobsPane, ok bool) {
	if !state.Maximized {
		return JobsPaneList, false
	}
	_, _, opsRect := SplitJobsSecondaryPanels(layout.Secondary, JobEntryShowsConflictPanel(sel), JobDetailLineCount(sel, now, throughputChartEnabled))
	pane = focusedJobsPane(state, JobsPaneOrder(JobEntryShowsConflictPanel(sel), opsRect.Height > 0))
	return pane, pane == JobsPaneOperations
}

// jobsViewRender carries the jobs view model fields drawJobsView reads.
type jobsViewRender struct {
	State                  JobsViewState
	Jobs                   []JobEntry
	Operations             map[string][]JobOperation
	FilterCaseInsensitive  bool
	UserHomeDir            string
	ThroughputChartEnabled bool
	RateLimitBPS           int64
}

func drawJobsView(screen tcell.Screen, layout Layout, v jobsViewRender, styles theme.Theme, now time.Time, chromeBlocked bool) {
	state, jobs := v.State, v.Jobs
	var sel JobEntry
	if state.Selected >= 0 && state.Selected < len(jobs) {
		sel = jobs[state.Selected]
	}
	ops := v.Operations[sel.ID]
	if _, ok := jobsMaximizedPane(layout, state, sel, now, v.ThroughputChartEnabled); ok {
		drawJobsOperationsPanel(screen, JobsMaximizedRect(layout), state, ops, v.FilterCaseInsensitive, styles, chromeBlocked, true, true)
		return
	}
	drawJobsListPanel(screen, layout.Primary, state, jobs, styles, now, chromeBlocked, v.RateLimitBPS)
	detailLines := JobDetailLineCount(sel, now, v.ThroughputChartEnabled)
	showConflict := JobEntryShowsConflictPanel(sel)
	conflictRect, detailRect, opsRect := SplitJobsSecondaryPanels(layout.Secondary, showConflict, detailLines)
	focused := focusedJobsPane(state, JobsPaneOrder(showConflict, opsRect.Height > 0))
	if conflictRect.Height > 0 {
		drawJobsConflictPanel(screen, conflictRect, state, sel, styles, chromeBlocked, focused == JobsPaneConflict, v.UserHomeDir)
	}
	drawJobsDetailPanel(screen, detailRect, state, jobs, styles, now, chromeBlocked, focused == JobsPaneDetail, v.UserHomeDir, v.ThroughputChartEnabled)
	if opsRect.Height > 0 {
		drawJobsOperationsPanel(screen, opsRect, state, ops, v.FilterCaseInsensitive, styles, chromeBlocked, focused == JobsPaneOperations, false)
	}
}

// formatRateLimitLabel renders the Queue panel border's rate-limit indicator: empty when
// unlimited (0), otherwise the throughput string padded for the border (e.g. " 20MB/s ").
// A free function, not a method on drawJobsListPanel, because that function's own
// "jobs []JobEntry" parameter shadows the internal/jobs package name.
func formatRateLimitLabel(bps int64) string {
	if bps <= 0 {
		return ""
	}
	return panelSelectionSizePadded(jobs.FormatThroughput(float64(bps)))
}

func drawJobsListPanel(screen tcell.Screen, rect Rect, state JobsViewState, jobs []JobEntry, styles theme.Theme, now time.Time, chromeBlocked bool, rateLimitBPS int64) {
	active := state.FocusPane == 0
	layout := drawAuxPanelChrome(screen, rect, " Queue ", formatRateLimitLabel(rateLimitBPS), active, chromeBlocked, false, styles)
	bg := layout.ContentBG

	contentX := rect.X + 2
	contentW := rect.Width - 4
	if contentW < 1 {
		contentW = 1
	}

	visibleRows := PanelListRows(rect)
	if visibleRows <= 0 || len(jobs) == 0 {
		if visibleRows > 0 {
			primitive.Text(screen, contentX, rect.Y+2, contentW, styles.EmptyText(), styles.PanelEmptyStyle().Background(bg))
		}
		return
	}

	hdr := fmt.Sprintf("%-3s%-*s %-10s%-10s%-10sProgress", "", jobsListColTypeRunes, "Type", "Status", "ETA", "Speed")
	headerStyle := auxPanelListHeaderStyle(layout.Chrome, chromeBlocked, bg)
	primitive.Text(screen, contentX, rect.Y+1, contentW, hdr, headerStyle)

	n := len(jobs)
	scroll := auxListScroll(state.ListScroll, n, visibleRows)

	rowBase := styles.JobsRow.Background(bg)
	for row := 0; row < visibleRows; row++ {
		idx := scroll + row
		y := rect.Y + 2 + row
		if idx >= n {
			break
		}
		entry := jobs[idx]
		selected := idx == state.Selected
		lineStyle := auxPanelListRowStyle(styles, rowBase, selected, chromeBlocked, active)
		_, lineBG, _ := lineStyle.Decompose()
		paintAuxPanelRowMargin(screen, rect.X+1, y, lineStyle)

		pct := jobPercentDone(entry)
		eta := formatJobETA(entry, now)
		statusStyle := styles.JobsStatusStyle(entry.Status).Background(lineBG)
		iconStyle := styles.JobsIconStyle(entry.Status)
		iconFG, _, iconAttrs := iconStyle.Decompose()
		iconRenderStyle := tcell.StyleDefault.Foreground(iconFG).Background(lineBG)
		if iconAttrs&tcell.AttrBold != 0 {
			iconRenderStyle = iconRenderStyle.Bold(true)
		}
		if iconAttrs&tcell.AttrUnderline != 0 {
			iconRenderStyle = iconRenderStyle.Underline(true)
		}
		if iconAttrs&tcell.AttrReverse != 0 {
			iconRenderStyle = iconRenderStyle.Reverse(true)
		}
		if selected {
			statusStyle = lineStyle
			iconRenderStyle = lineStyle
		}
		iconIcon := styles.IconJobsList(entry.Status)
		primitive.Text(screen, contentX, y, jobsListColIcon, iconIcon, iconRenderStyle)
		line := fmt.Sprintf("%-*s ", jobsListColTypeRunes, primitive.TruncateRight(entry.Type, jobsListColTypeRunes))
		primitive.Text(screen, contentX+jobsListColIcon, y, jobsListColTypeCell, line, lineStyle)
		xStatus := contentX + jobsListColPrefix
		primitive.Text(screen, xStatus, y, jobsListColStatus, primitive.TruncateRight(entry.Status, 9), statusStyle)
		xETA := xStatus + jobsListColStatus
		primitive.Text(screen, xETA, y, jobsListColETA, primitive.TruncateRight(eta, 9), lineStyle)
		xSpeed := xETA + jobsListColETA
		speedLabel := formatJobSpeed(entry, now)
		primitive.Text(screen, xSpeed, y, jobsListColSpeed, primitive.TruncateRight(speedLabel, jobsListColSpeed-1), lineStyle)
		xProg := xSpeed + jobsListColSpeed
		barW := rect.X + rect.Width - 1 - xProg // reach the border, no trailing margin
		if barW < 0 {
			barW = 0
		}
		// Leave a one-cell gap, unhighlighted even on the selected row, so the
		// cyan cursor background never touches the progress bar's own colors.
		paintAuxPanelRowMargin(screen, xProg-1, y, rowBase)
		drawJobsProgressBar(screen, xProg, y, barW, pct,
			styles.JobsProgressFill,
			styles.JobsProgressTrack,
			styles.JobsProgressLabelOnFill,
			styles.JobsProgressLabelOnTrack,
		)
	}
}

func drawJobsDetailPanel(screen tcell.Screen, rect Rect, state JobsViewState, jobs []JobEntry, styles theme.Theme, now time.Time, chromeBlocked bool, focused bool, userHomeDir string, throughputChartEnabled bool) {
	layout := drawAuxPanelChrome(screen, rect, " Details ", "", focused, chromeBlocked, false, styles)
	body := auxPanelBodyText(styles, chromeBlocked, layout.ContentBG)
	contentTop := rect.Y + 1
	contentH := rect.Height - 2
	if contentH <= 0 {
		return
	}

	textX := rect.X + 2
	textW := rect.Width - 4
	if textW < 1 {
		textW = 1
	}

	var sel JobEntry
	if state.Selected >= 0 && state.Selected < len(jobs) {
		sel = jobs[state.Selected]
	}

	all := detailStaticLines(sel, now, textW, userHomeDir, throughputChartEnabled)

	scroll := state.DetailScroll
	if scroll < 0 {
		scroll = 0
	}
	maxStart := max(0, len(all)-contentH)
	if scroll > maxStart {
		scroll = maxStart
	}

	for i := 0; i < contentH; i++ {
		line := ""
		if i+scroll < len(all) {
			line = all[i+scroll]
		}
		if sel.ID == "" && i+scroll == 1 { // empty-detail placeholder row
			primitive.Text(screen, textX, contentTop+i, textW, styles.EmptyText(), styles.PanelEmptyStyle().Background(layout.ContentBG))
			continue
		}
		primitive.Text(screen, textX, contentTop+i, textW, line, body)
	}
}

func jobsDetailPathBudget(pathMax int, linePrefix string) int {
	n := utf8.RuneCountInString(linePrefix)
	b := pathMax - n
	if b < 1 {
		return 1
	}
	return b
}

func jobFinishedWallDuration(j JobEntry) time.Duration {
	if j.StartedAt.IsZero() {
		return 0
	}
	end := j.FinishedAt
	if end.IsZero() || end.Before(j.StartedAt) {
		return 0
	}
	return end.Sub(j.StartedAt)
}

func detailDurationOrETALine(j JobEntry, now time.Time) string {
	if jobs.Status(j.Status).IsFinished() {
		label := jobs.FormatHumanDuration(jobFinishedWallDuration(j))
		if j.StartedAt.IsZero() || j.FinishedAt.IsZero() || j.FinishedAt.Before(j.StartedAt) {
			label = "—"
		}
		return fmt.Sprintf("Took:        %s", label)
	}
	return fmt.Sprintf("ETA:         %s", formatJobETAFull(j, now))
}

// jobDetailProgressLine formats the Details panel progress row. Byte totals are shown
// only when TotalBytes is known (copy/move scan). Delete jobs omit the ratio and may
// show cumulative deleted size instead.
func jobDetailProgressLine(j JobEntry) string {
	tfLabel := fmt.Sprintf("%d", j.TotalFiles)
	if j.TotalFiles <= 0 {
		tfLabel = "?"
	}
	line := fmt.Sprintf("Progress:    %d / %s items", j.DoneFiles, tfLabel)
	if j.TotalBytes > 0 {
		line += fmt.Sprintf("   %s / %s bytes", formatJobBytes(j.DoneBytes), formatJobBytes(j.TotalBytes))
	} else if j.Type == string(jobs.TypeDelete) && j.DoneBytes > 0 {
		line += fmt.Sprintf("   %s deleted", formatJobBytes(j.DoneBytes))
	}
	if j.TotalDirs > 0 || j.Status == "scanning" {
		tdLabel := "?"
		if j.TotalDirs > 0 {
			tdLabel = fmt.Sprintf("%d", j.TotalDirs)
		}
		line += fmt.Sprintf("   (%s dirs)", tdLabel)
	}
	if jobStillScanningInBackground(j) {
		line += "   (scanning…)"
	}
	return line
}

// jobStillScanningInBackground reports whether a copy/move/flatten job's background pre-scan
// (jobbridge.ScanFunc's counting walk) is still enumerating the source tree after the job has
// already left the "scanning" status and started transferring — meaning TotalFiles/TotalDirs/
// TotalBytes above are still provisional and will keep growing.
func jobStillScanningInBackground(j JobEntry) bool {
	if j.TotalsComplete || j.Status == "scanning" || jobs.Status(j.Status).IsFinished() {
		return false
	}
	switch jobs.Type(j.Type) {
	case jobs.TypeCopy, jobs.TypeMove, jobs.TypeFlatten:
		return true
	default:
		return false
	}
}

func detailStaticLines(j JobEntry, now time.Time, pathMax int, userHomeDir string, throughputChartEnabled bool) []string {
	if pathMax < 1 {
		pathMax = jobsDetailLineBudgetFallback
	}
	if j.ID == "" {
		return []string{"", "", ""} // middle row is drawn as the themed empty text
	}
	prefixDestination := "Destination: "
	prefixError := "Error:       "
	prefixSources := "Sources:     "
	prefixCurrent := "Current:     "
	lines := []string{
		fmt.Sprintf("Type:        %s", j.Type),
		fmt.Sprintf("Status:      %s", j.Status),
	}
	if j.Undo != "" {
		lines = append(lines, "Undo:        "+j.Undo)
	}
	if j.Error != "" {
		lines = append(lines, fmt.Sprintf(prefixError+"%s", primitive.TruncateMiddle(j.Error, jobsDetailPathBudget(pathMax, prefixError))))
	}
	if len(j.Warnings) > 0 {
		lines = append(lines, fmt.Sprintf("Warnings (%d):", len(j.Warnings)))
		for _, w := range j.Warnings {
			lines = append(lines, primitive.TruncateMiddle(w, pathMax))
		}
	}
	src := "—"
	if len(j.Sources) > 0 {
		srcBody := j.Sources[0]
		suffix := ""
		if len(j.Sources) > 1 {
			suffix = fmt.Sprintf(" (+%d more)", len(j.Sources)-1)
		}
		budget := jobsDetailPathBudget(pathMax, prefixSources) - utf8.RuneCountInString(suffix)
		if budget < 1 {
			budget = 1
		}
		displaySrc := primitive.PathWithHomeTilde(srcBody, userHomeDir)
		src = primitive.FitPathForWidth(displaySrc, budget) + suffix
	}
	lines = append(lines, fmt.Sprintf(prefixSources+"%s", src))
	if j.Type != string(jobs.TypeDelete) {
		dest := "—"
		if j.Destination != "" {
			destDisplay := primitive.PathWithHomeTilde(j.Destination, userHomeDir)
			dest = primitive.FitPathForWidth(destDisplay, jobsDetailPathBudget(pathMax, prefixDestination))
		}
		lines = append(lines, fmt.Sprintf(prefixDestination+"%s", dest))
	}
	if j.CurrentPath != "" {
		curDisplay := primitive.PathWithHomeTilde(j.CurrentPath, userHomeDir)
		lines = append(lines, fmt.Sprintf(prefixCurrent+"%s", primitive.FitPathForWidth(curDisplay, jobsDetailPathBudget(pathMax, prefixCurrent))))
	}
	lines = append(lines, jobDetailProgressLine(j))
	lines = append(lines, detailDurationOrETALine(j, now))
	if throughputChartEnabled {
		lines = append(lines, ThroughputDetailLines(j.ThroughputStrip, pathMax, j.Status == "running")...)
	}
	return lines
}

// JobDetailLineCount returns the number of lines in the jobs Details panel for scroll bounds.
func JobDetailLineCount(j JobEntry, now time.Time, throughputChartEnabled bool) int {
	return len(detailStaticLines(j, now, jobsDetailLineBudgetFallback, "", throughputChartEnabled))
}

// JobsLinesPanelLineCount returns the number of rows in an Errors or Succeeded panel (an empty
// Succeeded panel shows one placeholder row) for scroll bounds.
func JobsLinesPanelLineCount(lines []string) int {
	return max(len(lines), 1)
}

func jobPercentDone(j JobEntry) float64 {
	if j.Status == "completed" {
		return 100
	}
	if j.TotalBytes > 0 {
		p := 100 * float64(j.DoneBytes) / float64(j.TotalBytes)
		if p > 100 {
			return 100
		}
		if p < 0 {
			return 0
		}
		return p
	}
	if j.TotalFiles <= 0 {
		return 0
	}
	p := 100 * float64(j.DoneFiles) / float64(j.TotalFiles)
	if p > 100 {
		return 100
	}
	return p
}

// drawJobsProgressBar paints a progress strip where remaining width after fixed columns is used.
// The percentage label is centered; icons over the filled portion use labelOnFill, else labelOnTrack.
func drawJobsProgressBar(screen tcell.Screen, x, y, width int, pct float64,
	fillStyle, trackStyle, labelOnFill, labelOnTrack tcell.Style,
) {
	if width <= 0 {
		return
	}
	p := pct
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	filledCols := int(float64(width)*p/100.0 + 0.5)
	if filledCols > width {
		filledCols = width
	}
	label := fmt.Sprintf("%3.0f%%", p)
	labelRunes := []rune(label)
	labelLen := len(labelRunes)
	startCol := (width - labelLen) / 2
	if startCol < 0 {
		startCol = 0
	}

	for col := 0; col < width; col++ {
		barStyle := trackStyle
		if col < filledCols {
			barStyle = fillStyle
		}
		ch := ' '
		labelIdx := col - startCol
		if labelIdx >= 0 && labelIdx < labelLen {
			ch = labelRunes[labelIdx]
		}
		st := barStyle
		if ch != ' ' {
			lbl := labelOnTrack
			if col < filledCols {
				lbl = labelOnFill
			}
			st = composeJobsProgressLabelCell(barStyle, lbl)
		}
		screen.SetContent(x+col, y, ch, nil, st)
	}
}

func composeJobsProgressLabelCell(barCell tcell.Style, labelStyle tcell.Style) tcell.Style {
	_, bg, _ := barCell.Decompose()
	fg, _, attrs := labelStyle.Decompose()
	out := tcell.StyleDefault.Foreground(fg).Background(bg)
	if attrs&tcell.AttrBold != 0 {
		out = out.Bold(true)
	}
	if attrs&tcell.AttrUnderline != 0 {
		out = out.Underline(true)
	}
	return out
}

// formatJobETAFull returns the computed ETA string without column truncation (for the Details panel).
func formatJobETAFull(j JobEntry, now time.Time) string {
	if j.QueueETA != "" {
		return j.QueueETA
	}
	return jobs.FormatETA(jobs.Status(j.Status), j.StartedAt, now, j.TotalBytes, j.DoneBytes, j.TotalFiles, j.DoneFiles, j.ETABytesPerSec, j.ETAFilesPerSec)
}

// formatJobETA returns a shortened ETA for the Queue list column (jobsListColETA).
func formatJobETA(j JobEntry, now time.Time) string {
	return primitive.TruncateRight(formatJobETAFull(j, now), jobsListColETA-1)
}

func formatJobSpeed(j JobEntry, now time.Time) string {
	bps := jobs.EffectiveDisplayThroughputBPS(jobs.Status(j.Status), j.StartedAt, now, j.DoneBytes, j.DisplaySpeedBPS)
	return jobs.FormatThroughput(bps)
}

func formatJobBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(n)/1024)
	}
	return fmt.Sprintf("%.1fM", float64(n)/(1024*1024))
}
