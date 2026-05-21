package ui

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

const jobActivityMaxLines = 200

// jobsDetailLineBudgetFallback is used when line width is unknown (layout line counts only).
const jobsDetailLineBudgetFallback = 4096

// Jobs list column layout to the Status column: icon, type (widest jobs.Type is "delete"), separator space.
const (
	jobsListColIcon      = 2
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
	ViewMessages
	ViewFilePreview
)

// IsAuxiliaryView reports vm is a full-screen meta view rather than the file browser.
func IsAuxiliaryView(vm ViewMode) bool {
	return vm == ViewJobs || vm == ViewCommands || vm == ViewMessages || vm == ViewFilePreview
}

// PanelZoomSplitsColumns reports whether the browser uses an asymmetric column split
// (Model.PanelZoomEnabled, from [ui].zoom_active_panel plus session override, preview suppression, and width gate).
// It applies only to the twin file browser; jobs/commands/messages use an even split.
func PanelZoomSplitsColumns(vm ViewMode, zoomEnabled bool) bool {
	return zoomEnabled && !IsAuxiliaryView(vm)
}

// JobsViewState holds focus and scroll positions for the jobs view screen.
type JobsViewState struct {
	Selected       int
	FocusPane      int // 0=list; 1=conflict when visible else details; 2=details or activity; 3=activity when conflict visible
	ListScroll     int
	DetailScroll   int
	ActivityScroll int
	// ConflictButtonFocus is the focused action index in the conflict panel (0..4).
	ConflictButtonFocus int
}

// Truncate job activity logs to avoid unbounded memory in long sessions.
func CapJobActivityLines(lines []string) []string {
	if len(lines) <= jobActivityMaxLines {
		return lines
	}
	return lines[len(lines)-jobActivityMaxLines:]
}

func drawJobsView(
	screen tcell.Screen,
	layout Layout,
	state JobsViewState,
	jobs []JobEntry,
	activity map[string][]string,
	styles theme.Theme,
	now time.Time,
	chromeBlocked bool,
	userHomeDir string,
	throughputChartEnabled bool,
) {
	drawJobsListPanel(screen, layout.Left, state, jobs, styles, now, chromeBlocked)
	var sel JobEntry
	if state.Selected >= 0 && state.Selected < len(jobs) {
		sel = jobs[state.Selected]
	}
	detailLines := JobDetailLineCount(sel, now, throughputChartEnabled)
	showConflict := JobEntryShowsConflictPanel(sel)
	conflictRect, detailRect, activityRect := SplitJobsRightPanels(layout.Right, showConflict, detailLines)
	detailFocused := jobsDetailPaneFocused(state, showConflict)
	activityFocused := jobsActivityPaneFocused(state, showConflict)
	conflictFocused := jobsConflictPaneFocused(state, showConflict)
	if conflictRect.Height > 0 {
		drawJobsConflictPanel(screen, conflictRect, state, sel, styles, chromeBlocked, conflictFocused, userHomeDir)
	}
	drawJobsDetailPanel(screen, detailRect, state, jobs, styles, now, chromeBlocked, detailFocused, userHomeDir, throughputChartEnabled)
	if activityRect.Height > 0 {
		drawJobsActivityPanel(screen, activityRect, state, jobs, activity, styles, chromeBlocked, activityFocused)
	}
}

func drawJobsListPanel(screen tcell.Screen, rect Rect, state JobsViewState, jobs []JobEntry, styles theme.Theme, now time.Time, chromeBlocked bool) {
	_, bg, _ := styles.PanelActiveSurface.Decompose()
	if chromeBlocked {
		_, bg, _ = styles.PanelBlockedSurface.Decompose()
	}
	active := state.FocusPane == 0
	var titleStyle tcell.Style
	var borderStyle tcell.Style
	if chromeBlocked {
		borderStyle = styles.PanelBlockedFrame
		titleStyle = styles.PanelBlockedTitle
	} else if active {
		borderStyle = styles.PanelActiveFrame
		titleStyle = styles.PanelActiveTitle
	} else {
		borderStyle = styles.PanelInactiveFrame
		titleStyle = styles.PanelInactiveTitle
	}
	primitive.Box(screen, primitive.Rect(rect), borderStyle)
	inner := primitive.Rect{X: rect.X + 1, Y: rect.Y + 1, Width: rect.Width - 2, Height: rect.Height - 2}
	if inner.Width > 0 && inner.Height > 0 {
		var surface tcell.Style
		if chromeBlocked {
			surface = styles.PanelBlockedSurface
		} else if active {
			surface = styles.PanelActiveSurface
		} else {
			surface = styles.PanelInactiveSurface
		}
		primitive.Fill(screen, inner, ' ', surface)
	}
	titleX := rect.X + 2
	titleWidth := rect.Width - 4
	primitive.TextOverlay(screen, titleX, rect.Y, titleWidth, " Queue ", titleStyle)

	contentX := rect.X + 2
	contentW := rect.Width - 4
	if contentW < 1 {
		contentW = 1
	}

	visibleRows := PanelListRows(rect)
	if visibleRows <= 0 || len(jobs) == 0 {
		if visibleRows > 0 {
			emptyStyle := styles.JobsRow.Background(bg)
			primitive.Text(screen, contentX, rect.Y+2, contentW, " No jobs ", emptyStyle)
		}
		return
	}

	hdr := fmt.Sprintf("%-2s%-*s %-10s%-10s%-10sProgress", "", jobsListColTypeRunes, "Type", "Status", "ETA", "Speed")
	headerStyle := styles.PanelActiveHeader.Background(bg)
	if chromeBlocked {
		headerStyle = styles.PanelBlockedHeader
	} else if !active {
		headerStyle = styles.PanelInactiveHeader.Background(bg)
	}
	primitive.Text(screen, contentX, rect.Y+1, contentW, hdr, headerStyle)

	n := len(jobs)
	scroll := state.ListScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll > 0 && scroll >= n {
		scroll = max(0, n-visibleRows)
	}
	if scroll+visibleRows > n {
		scroll = max(0, n-visibleRows)
	}

	for row := 0; row < visibleRows; row++ {
		idx := scroll + row
		y := rect.Y + 2 + row
		if idx >= n {
			break
		}
		entry := jobs[idx]
		lineStyle := styles.JobsRow.Background(bg)
		if idx == state.Selected {
			if chromeBlocked {
				lineStyle = styles.PanelBlockedCursor
			} else if active {
				lineStyle = styles.PanelRowSelected.Background(bg)
			} else {
				lineStyle = styles.PanelCursorInactive.Background(bg)
			}
		}

		pct := jobPercentDone(entry)
		eta := formatJobETA(entry, now)
		statusStyle := jobStatusStyle(entry.Status, styles).Background(bg)
		iconStyle := jobIconStyle(entry.Status, styles)
		_, lineBG, _ := lineStyle.Decompose()
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
		iconGlyph := jobRowLeadingIcon(entry.Status, styles)
		primitive.Text(screen, contentX, y, 2, iconGlyph, iconRenderStyle)
		line := fmt.Sprintf("%-*s ", jobsListColTypeRunes, truncateRunes(entry.Type, jobsListColTypeRunes))
		primitive.Text(screen, contentX+jobsListColIcon, y, jobsListColTypeCell, line, lineStyle)
		xStatus := contentX + jobsListColPrefix
		primitive.Text(screen, xStatus, y, jobsListColStatus, truncateRunes(entry.Status, 9), statusStyle)
		xETA := xStatus + jobsListColStatus
		primitive.Text(screen, xETA, y, jobsListColETA, truncateRunes(eta, 9), lineStyle)
		xSpeed := xETA + jobsListColETA
		speedLabel := formatJobSpeed(entry, now)
		primitive.Text(screen, xSpeed, y, jobsListColSpeed, truncateRunes(speedLabel, jobsListColSpeed-1), lineStyle)
		xProg := xSpeed + jobsListColSpeed
		barW := contentW - jobsListColPrefix - jobsListColStatus - jobsListColETA - jobsListColSpeed
		if barW < 0 {
			barW = 0
		}
		drawJobsProgressBar(screen, xProg, y, barW, pct,
			styles.JobsProgressFill,
			styles.JobsProgressTrack,
			styles.JobsProgressLabelOnFill,
			styles.JobsProgressLabelOnTrack,
		)
	}
}

func drawJobsDetailPanel(screen tcell.Screen, rect Rect, state JobsViewState, jobs []JobEntry, styles theme.Theme, now time.Time, chromeBlocked bool, focused bool, userHomeDir string, throughputChartEnabled bool) {
	_, bg, _ := styles.PanelActiveSurface.Decompose()
	if chromeBlocked {
		_, bg, _ = styles.PanelBlockedSurface.Decompose()
	}
	active := focused
	var titleStyle tcell.Style
	var borderStyle tcell.Style
	if chromeBlocked {
		borderStyle = styles.PanelBlockedFrame
		titleStyle = styles.PanelBlockedTitle
	} else if active {
		borderStyle = styles.PanelActiveFrame
		titleStyle = styles.PanelActiveTitle
	} else {
		borderStyle = styles.PanelInactiveFrame
		titleStyle = styles.PanelInactiveTitle
	}
	primitive.Box(screen, primitive.Rect(rect), borderStyle)
	inner := primitive.Rect{X: rect.X + 1, Y: rect.Y + 1, Width: rect.Width - 2, Height: rect.Height - 2}
	if inner.Width > 0 && inner.Height > 0 {
		var surface tcell.Style
		if chromeBlocked {
			surface = styles.PanelBlockedSurface
		} else if active {
			surface = styles.PanelActiveSurface
		} else {
			surface = styles.PanelInactiveSurface
		}
		primitive.Fill(screen, inner, ' ', surface)
	}
	titleX := rect.X + 2
	titleWidth := rect.Width - 4
	primitive.TextOverlay(screen, titleX, rect.Y, titleWidth, " Details ", titleStyle)

	body := styles.PanelText.Background(bg)
	if chromeBlocked {
		body = styles.PanelBlockedText
	}
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
		primitive.Text(screen, textX, contentTop+i, textW, line, body)
	}
}

func drawJobsActivityPanel(screen tcell.Screen, rect Rect, state JobsViewState, jobs []JobEntry, activity map[string][]string, styles theme.Theme, chromeBlocked bool, focused bool) {
	_, bg, _ := styles.PanelActiveSurface.Decompose()
	if chromeBlocked {
		_, bg, _ = styles.PanelBlockedSurface.Decompose()
	}
	active := focused
	var titleStyle tcell.Style
	var borderStyle tcell.Style
	if chromeBlocked {
		borderStyle = styles.PanelBlockedFrame
		titleStyle = styles.PanelBlockedTitle
	} else if active {
		borderStyle = styles.PanelActiveFrame
		titleStyle = styles.PanelActiveTitle
	} else {
		borderStyle = styles.PanelInactiveFrame
		titleStyle = styles.PanelInactiveTitle
	}
	primitive.Box(screen, primitive.Rect(rect), borderStyle)
	inner := primitive.Rect{X: rect.X + 1, Y: rect.Y + 1, Width: rect.Width - 2, Height: rect.Height - 2}
	if inner.Width > 0 && inner.Height > 0 {
		var surface tcell.Style
		if chromeBlocked {
			surface = styles.PanelBlockedSurface
		} else if active {
			surface = styles.PanelActiveSurface
		} else {
			surface = styles.PanelInactiveSurface
		}
		primitive.Fill(screen, inner, ' ', surface)
	}
	titleX := rect.X + 2
	titleWidth := rect.Width - 4
	primitive.TextOverlay(screen, titleX, rect.Y, titleWidth, " Activity ", titleStyle)

	body := styles.PanelText.Background(bg)
	if chromeBlocked {
		body = styles.PanelBlockedText
	}
	contentTop := rect.Y + 1
	contentH := rect.Height - 2
	if contentH <= 0 {
		return
	}

	var sel JobEntry
	if state.Selected >= 0 && state.Selected < len(jobs) {
		sel = jobs[state.Selected]
	}
	act := activity[sel.ID]
	var lines []string
	if len(act) == 0 {
		lines = []string{"", " No activity"}
	} else {
		lines = append(lines, act...)
	}

	scroll := state.ActivityScroll
	if scroll < 0 {
		scroll = 0
	}
	maxStart := max(0, len(lines)-contentH)
	if scroll > maxStart {
		scroll = maxStart
	}

	textX := rect.X + 2
	textW := rect.Width - 4
	if textW < 1 {
		textW = 1
	}
	for i := 0; i < contentH; i++ {
		line := ""
		if i+scroll < len(lines) {
			line = lines[i+scroll]
		}
		if line != "" && line != " No activity" {
			line = primitive.FitPathForWidth(line, textW)
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
		return fmt.Sprintf(" Took:        %s", label)
	}
	return fmt.Sprintf(" ETA:         %s", formatJobETAFull(j, now))
}

func detailStaticLines(j JobEntry, now time.Time, pathMax int, userHomeDir string, throughputChartEnabled bool) []string {
	if pathMax < 1 {
		pathMax = jobsDetailLineBudgetFallback
	}
	if j.ID == "" {
		return []string{" No job selected"}
	}
	prefixDestination := " Destination: "
	prefixError := " Error:       "
	prefixSources := " Sources:     "
	prefixCurrent := " Current:     "
	lines := []string{
		fmt.Sprintf(" Type:        %s", j.Type),
		fmt.Sprintf(" Status:      %s", j.Status),
	}
	if j.Error != "" {
		lines = append(lines, fmt.Sprintf(prefixError+"%s", truncateMiddle(j.Error, jobsDetailPathBudget(pathMax, prefixError))))
	}
	src := " —"
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
	dest := " —"
	if j.Destination != "" {
		destDisplay := primitive.PathWithHomeTilde(j.Destination, userHomeDir)
		dest = primitive.FitPathForWidth(destDisplay, jobsDetailPathBudget(pathMax, prefixDestination))
	}
	lines = append(lines, fmt.Sprintf(prefixDestination+"%s", dest))
	if j.CurrentPath != "" {
		curDisplay := primitive.PathWithHomeTilde(j.CurrentPath, userHomeDir)
		lines = append(lines, fmt.Sprintf(prefixCurrent+"%s", primitive.FitPathForWidth(curDisplay, jobsDetailPathBudget(pathMax, prefixCurrent))))
	}
	tf := j.TotalFiles
	tfLabel := fmt.Sprintf("%d", tf)
	if tf <= 0 {
		tfLabel = "?"
	}
	progressLine := fmt.Sprintf(" Progress:    %d / %s items   %s / %s bytes",
		j.DoneFiles, tfLabel, formatJobBytes(j.DoneBytes), formatJobBytes(j.TotalBytes))
	if j.TotalDirs > 0 || j.Status == "scanning" {
		tdLabel := "?"
		if j.TotalDirs > 0 {
			tdLabel = fmt.Sprintf("%d", j.TotalDirs)
		}
		progressLine += fmt.Sprintf("   (%s dirs)", tdLabel)
	}
	lines = append(lines, progressLine)
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

// JobActivityLineCount returns the number of lines in the jobs Activity panel for scroll bounds.
func JobActivityLineCount(activity []string) int {
	if len(activity) == 0 {
		return 1
	}
	return len(activity)
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
// The percentage label is centered; glyphs over the filled portion use labelOnFill, else labelOnTrack.
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
	return truncateRunes(formatJobETAFull(j, now), jobsListColETA-1)
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

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

func truncateMiddle(s string, max int) string {
	r := []rune(s)
	if len(r) <= max || max <= 3 {
		return truncateRunes(s, max)
	}
	left := max/2 - 1
	right := max - left - 1
	return string(r[:left]) + "…" + string(r[len(r)-right:])
}

// jobRowLeadingIcon returns the Nerd Font glyph for the given job status.
// Icons follow the status definitions in AGENTS.md:
//
//	  Ongoing (queued / running)
//	  Paused
//	  Stopped (canceled)
//	  Error (failed)
//	󰋗  Input required (decision)
//	  Completed
func jobRowLeadingIcon(status string, th theme.Theme) string {
	if sym, ok := th.Symbols[status]; ok && sym != "" {
		return sym
	}
	switch status {
	case "scanning":
		return "\uf110" //  spinner
	case "queued":
		return "\u231B" // ⏳ Queued (clock face)
	case "running":
		return "\uf144" //  Ongoing (play circle)
	case "decision":
		return "\U000f02d7" // 󰋗 Input required
	case "paused":
		return "\uf28b" //  Paused (pause circle)
	case "canceled":
		return "\uf28d" //  Stopped (stop circle)
	case "failed":
		return "\uf06a" //  Error (exclamation triangle)
	case "completed":
		return "\uf05d" //  Completed
	default:
		return " "
	}
}

// jobIconStyle returns the themed style for the leading icon of a job status.
func jobIconStyle(status string, styles theme.Theme) tcell.Style {
	switch status {
	case "scanning":
		return styles.JobsIconsScanning
	case "queued":
		return styles.JobsIconsQueued
	case "running":
		return styles.JobsIconsOngoing
	case "paused":
		return styles.JobsIconsPaused
	case "canceled":
		return styles.JobsIconsStopped
	case "failed":
		return styles.JobsIconsError
	case "decision":
		return styles.JobsIconsInputRequired
	case "completed":
		return styles.JobsIconsCompleted
	default:
		return styles.JobsRow
	}
}

func jobStatusStyle(status string, styles theme.Theme) tcell.Style {
	switch status {
	case "running", "queued", "scanning", "paused", "decision":
		return styles.JobsRunning
	case "completed":
		return styles.JobsDone
	case "failed", "canceled":
		return styles.JobsFailed
	default:
		return styles.JobsRow
	}
}
