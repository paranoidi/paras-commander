package ui

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/oplog"
	"github.com/paranoidi/paras-commander/internal/panellist"
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
	jobsListColTypeRunes = 9 // internal/jobs.Type: extract (longest), a space, the sources expander icon
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
	// Selected is the selected job's index; SelectedSub is 0 on its job row and n on the
	// sub-row of source n-1 (see SelectedSource). Expanded holds the IDs of jobs showing their
	// source sub-rows (JobsListRows).
	Selected     int
	SelectedSub  int
	Expanded     map[string]bool
	FocusPane    int // index into JobsPaneOrder: list, blocker (if shown), Details, Operations (if shown)
	ListScroll   int
	DetailScroll int
	OpsScroll    int
	// OpsResultHidden is the set of results the Operations pane hides (zero shows all; see
	// OpsResultShown). OpsFilter holds the Operations fuzzy query; only its Query, Cursor and
	// Editing are used (the jobs handler matches the operation log itself). OpsScroll is the
	// newest-first display row at the top of the pane.
	OpsResultHidden dialog.JobResultFilter
	OpsFilter       quickfilter.Filter
	// ConflictButtonFocus is the focused button index in the blocker panel (JobsBlockerMaxButtonIndex).
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
	OpsWindow              JobsOpsWindow
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
	win := v.OpsWindow
	if win.JobID != sel.ID {
		win = JobsOpsWindow{JobID: sel.ID}
	}
	sourceLabel := ""
	if src := state.SelectedSource(jobs); src != oplog.NoSource {
		sourceLabel = jobSourceLabel(sel.Sources, src)
	}
	if _, ok := jobsMaximizedPane(layout, state, sel, now, v.ThroughputChartEnabled); ok {
		drawJobsOperationsPanel(screen, JobsMaximizedRect(layout), state, win, sourceLabel, styles, chromeBlocked, true, true)
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
		drawJobsOperationsPanel(screen, opsRect, state, win, sourceLabel, styles, chromeBlocked, focused == JobsPaneOperations, false)
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

	rows := state.JobsListRows(jobs)
	n := len(rows)
	scroll := auxListScroll(state.ListScroll, n, visibleRows)
	selectedRow := state.SelectedRow(rows)

	rowBase := styles.JobsRow.Background(bg)
	for row := 0; row < visibleRows; row++ {
		idx := scroll + row
		y := rect.Y + 2 + row
		if idx >= n {
			break
		}
		r := rows[idx]
		entry := jobs[r.Job]
		selected := idx == selectedRow
		lineStyle := auxPanelListRowStyle(styles, rowBase, selected, chromeBlocked, active)
		paintAuxPanelRowMargin(screen, rect.X+1, y, lineStyle)
		xProg := contentX + jobsListColPrefix + jobsListColStatus + jobsListColETA + jobsListColSpeed
		barW := max(rect.X+rect.Width-1-xProg, 0) // reach the border, no trailing margin
		// Leave a one-cell gap, unhighlighted even on the selected row, so the
		// cyan cursor background never touches the progress bar's own colors.
		paintAuxPanelRowMargin(screen, xProg-1, y, rowBase)
		if r.Source != oplog.NoSource {
			last := r.Source == len(entry.Sources)-1
			drawJobsListSourceRow(screen, contentX, y, xProg, barW, entry, r.Source, last, lineStyle, selected, chromeBlocked, styles)
			continue
		}
		drawJobsListJobRow(screen, contentX, y, xProg, barW, entry, state.JobExpanded(entry), lineStyle, selected, styles, now)
	}
}

// jobsListIconStyle is a Queue status icon's style on a row background: the icon's foreground
// and attributes, or the whole selected-row style on the cursor row.
func jobsListIconStyle(styles theme.Theme, status string, lineStyle tcell.Style, selected bool) tcell.Style {
	if selected {
		return lineStyle
	}
	_, lineBG, _ := lineStyle.Decompose()
	iconFG, _, iconAttrs := styles.JobsIconStyle(status).Decompose()
	st := tcell.StyleDefault.Foreground(iconFG).Background(lineBG)
	if iconAttrs&tcell.AttrBold != 0 {
		st = st.Bold(true)
	}
	if iconAttrs&tcell.AttrUnderline != 0 {
		st = st.Underline(true)
	}
	if iconAttrs&tcell.AttrReverse != 0 {
		st = st.Reverse(true)
	}
	return st
}

// jobsListTypeLabel is the Type cell text: the job type, then the expander icon for a job
// with source sub-rows.
func jobsListTypeLabel(entry JobEntry, expanded bool, styles theme.Theme) string {
	if !JobEntryHasSources(entry) {
		return entry.Type
	}
	return entry.Type + " " + styles.IconJobsListSources(expanded)
}

func drawJobsListJobRow(screen tcell.Screen, contentX, y, xProg, barW int, entry JobEntry, expanded bool, lineStyle tcell.Style, selected bool, styles theme.Theme, now time.Time) {
	_, lineBG, _ := lineStyle.Decompose()
	statusStyle := styles.JobsStatusStyle(entry.Status).Background(lineBG)
	if selected {
		statusStyle = lineStyle
	}
	primitive.Text(screen, contentX, y, jobsListColIcon, styles.IconJobsList(entry.Status), jobsListIconStyle(styles, entry.Status, lineStyle, selected))
	line := fmt.Sprintf("%-*s ", jobsListColTypeRunes, primitive.TruncateRight(jobsListTypeLabel(entry, expanded, styles), jobsListColTypeRunes))
	primitive.Text(screen, contentX+jobsListColIcon, y, jobsListColTypeCell, line, lineStyle)
	xStatus := contentX + jobsListColPrefix
	primitive.Text(screen, xStatus, y, jobsListColStatus, primitive.TruncateRight(entry.Status, 9), statusStyle)
	xETA := xStatus + jobsListColStatus
	primitive.Text(screen, xETA, y, jobsListColETA, primitive.TruncateRight(formatJobETA(entry, now), 9), lineStyle)
	xSpeed := xETA + jobsListColETA
	primitive.Text(screen, xSpeed, y, jobsListColSpeed, primitive.TruncateRight(formatJobSpeed(entry, now), jobsListColSpeed-1), lineStyle)
	progFill, progTrack, progLabelOnFill, progLabelOnTrack := styles.JobsProgressStyles(entry.Status)
	drawJobsProgressBar(screen, xProg, y, barW, jobPercentDone(entry), progFill, progTrack, progLabelOnFill, progLabelOnTrack)
}

// drawJobsListSourceRow paints the sub-row of source src under an expanded job: the tree
// connector in the icon column, the source's status icon and label across Type..Speed, and
// its done / failed / skipped counts in the Progress column.
func drawJobsListSourceRow(screen tcell.Screen, contentX, y, xProg, countsW int, entry JobEntry, src int, last bool, lineStyle tcell.Style, selected, chromeBlocked bool, styles theme.Theme) {
	_, lineBG, _ := lineStyle.Decompose()
	connectorStyle := styles.PanelRowTreeConnector.Background(lineBG)
	if selected {
		connectorStyle = lineStyle
	}
	primitive.Text(screen, contentX, y, jobsListColIcon, panellist.TreeConnectorPrefix(1, last, nil, styles), connectorStyle)
	status := jobSourceStatus(entry, src)
	x := contentX + jobsListColIcon
	icon := styles.IconJobsList(status)
	primitive.Text(screen, x, y, 2, icon+" ", jobsListIconStyle(styles, status, lineStyle, selected))
	labelW := max(xProg-1-(x+2), 0)
	primitive.Text(screen, x+2, y, labelW, primitive.FitPathForWidth(jobSourceLabel(entry.Sources, src), labelW), lineStyle)
	var stats oplog.Stats
	if src < len(entry.SourceStats) {
		stats = entry.SourceStats[src]
	}
	parts := jobResultCountsParts(stats, "")
	text := ""
	for _, p := range parts {
		text += p.Text
	}
	body := styles.JobsRow.Background(lineBG)
	if selected {
		body = lineStyle
	}
	var spans []primitive.Span
	if !chromeBlocked && !selected {
		spans = jobFilesPartSpans(parts, styles, body)
	}
	primitive.StyledText(screen, xProg, y, countsW, text, body, spans)
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
		lineStyle := body
		var spans []primitive.Span
		if !chromeBlocked {
			lineStyle = styles.JobsDetailText.Background(layout.ContentBG)
			if isJobsDetailPlotLine(line) {
				lineStyle = styles.JobsDetailPlot.Background(layout.ContentBG)
			} else if n := jobsDetailLabelRunes(line); n > 0 {
				spans = append(spans, primitive.Span{Start: 0, End: n, Style: styles.JobsDetailLabel.Background(layout.ContentBG)})
			}
			if strings.HasPrefix(line, jobDetailFilesPrefix) {
				spans = append(spans, jobFilesLineSpans(sel, styles, lineStyle)...)
			}
		}
		primitive.StyledText(screen, textX, contentTop+i, textW, line, lineStyle, spans)
	}
}

// isJobsDetailPlotLine reports whether a Details line is a throughput chart row (holds braille dots).
func isJobsDetailPlotLine(line string) bool {
	for _, r := range line {
		if r >= '\u2801' && r <= '\u28ff' {
			return true
		}
	}
	return false
}

// jobsDetailLabelRunes returns the rune length of a leading "Label:" plus its padding spaces, or 0
// when the line has no label.
func jobsDetailLabelRunes(line string) int {
	runes := []rune(line)
	colon := slices.Index(runes, ':')
	if colon < 0 {
		return 0
	}
	end := colon + 1
	for end < len(runes) && runes[end] == ' ' {
		end++
	}
	if end == colon+1 {
		return 0
	}
	return end
}

// jobDetailFilesPrefix starts the Details line with the job's operation counts.
const jobDetailFilesPrefix = "Files:       "

// jobFilesPart is one piece of the Details Files line; Key, when set, is the result theme key
// that colors it.
type jobFilesPart struct {
	Text string
	Key  string
}

// jobFilesLineParts is the Details "Files:" line (done, then failed and skipped when non-zero,
// then what a canceled job left undone, then a note when the log dropped rows) split into its
// colored parts: the one source of the line's text and its spans. nil when the job has recorded
// nothing and was not canceled.
func jobFilesLineParts(j JobEntry) []jobFilesPart {
	st := j.OpsStats
	recorded := st.Done + st.Failed + st.Skipped
	canceled := jobs.Status(j.Status) == jobs.StatusCanceled
	if recorded == 0 && !canceled {
		return nil
	}
	parts := jobResultCountsParts(st, jobDetailFilesPrefix)
	if canceled {
		text := ""
		switch n, count := jobs.CanceledItems(jobs.Type(j.Type), j.TotalFiles, j.TotalDirs, j.TotalsComplete, j.ProcessedItems, recorded); {
		case count == jobs.CanceledUnknown:
			text = "canceled: incomplete scan"
		case count == jobs.CanceledUnsupported:
			text = "canceled: unsupported"
		case n > 0:
			text = fmt.Sprintf("%d canceled", n)
		}
		if text != "" {
			parts = append(parts, jobFilesPart{Text: ", "}, jobFilesPart{Text: text, Key: theme.JobsOperationSkipped})
		}
	}
	if st.Dropped > 0 {
		parts = append(parts, jobFilesPart{Text: fmt.Sprintf(" (log incomplete, %d dropped)", st.Dropped)})
	}
	return parts
}

// jobResultCountsParts is "N done", then ", N failed" and ", N skipped" when non-zero, after
// prefix: the counts shared by the Details Files line and the Queue source sub-rows.
func jobResultCountsParts(st oplog.Stats, prefix string) []jobFilesPart {
	parts := []jobFilesPart{{Text: fmt.Sprintf("%s%d done", prefix, st.Done)}}
	for _, c := range []struct {
		n    int
		word string
		key  string
	}{{st.Failed, "failed", theme.JobsOperationFailed}, {st.Skipped, "skipped", theme.JobsOperationSkipped}} {
		if c.n > 0 {
			parts = append(parts, jobFilesPart{Text: ", "}, jobFilesPart{Text: fmt.Sprintf("%d %s", c.n, c.word), Key: c.key})
		}
	}
	return parts
}

// jobFilesLineSpans colors the failed and skipped counts of the Details Files line.
func jobFilesLineSpans(j JobEntry, styles theme.Theme, body tcell.Style) []primitive.Span {
	return jobFilesPartSpans(jobFilesLineParts(j), styles, body)
}

// jobFilesPartSpans colors the parts that carry a result key.
func jobFilesPartSpans(parts []jobFilesPart, styles theme.Theme, body tcell.Style) []primitive.Span {
	var spans []primitive.Span
	pos := 0
	for _, p := range parts {
		n := utf8.RuneCountInString(p.Text)
		if p.Key != "" {
			spans = append(spans, primitive.Span{Start: pos, End: pos + n, Style: styles.JobsOperationOn(p.Key, body)})
		}
		pos += n
	}
	return spans
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
	if jobTotalsEstimated(j) {
		line += "   (estimated)"
	} else if jobStillScanningInBackground(j) {
		line += "   (scanning…)"
	}
	return line
}

// jobTotalsEstimated reports whether TotalBytes is still the enqueue-time estimate
// (jobs.Job.EstimatedBytes): a copy until its counting walk finishes, a move (no counting walk)
// for its whole run.
func jobTotalsEstimated(j JobEntry) bool {
	if j.EstimatedBytes <= 0 || jobs.Status(j.Status).IsFinished() {
		return false
	}
	switch jobs.Type(j.Type) {
	case jobs.TypeMove:
		return true
	case jobs.TypeCopy:
		return !j.TotalsComplete
	default:
		return false
	}
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
	prefixVerifying := "Verifying:   "
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
	if parts := jobFilesLineParts(j); parts != nil {
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(p.Text)
		}
		lines = append(lines, b.String())
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
	if j.VerifyTotal > 0 {
		lines = append(lines, fmt.Sprintf(prefixVerifying+"%s / %s (%d%%)", formatJobBytes(j.VerifyDone), formatJobBytes(j.VerifyTotal), 100*j.VerifyDone/j.VerifyTotal))
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
