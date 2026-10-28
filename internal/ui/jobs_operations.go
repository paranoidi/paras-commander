package ui

import (
	"slices"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/oplog"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// JobOperation is one row of the jobs view Operations pane: an item the job processed, its
// result and, for skipped or failed items, why.
type JobOperation struct {
	Result        oplog.Result
	Label         string
	Reason        string
	SourceDeleted bool
}

// JobOperationFromRecord converts an operation log record to a pane row.
func JobOperationFromRecord(r oplog.Record) JobOperation {
	return JobOperation{Result: r.Result, Label: r.Label, Reason: r.Reason, SourceDeleted: r.SourceDeleted}
}

// Text is the row's displayed text: the label, plus ": <reason>" when it has one.
func (o JobOperation) Text() string {
	if o.Reason != "" {
		return o.Label + ": " + o.Reason
	}
	return o.Label
}

// JobOperationResultKey maps a result to its theme key ([jobs.operations], [icons.jobs.operations]).
func JobOperationResultKey(r oplog.Result) string {
	switch r {
	case oplog.Failed:
		return theme.JobsOperationFailed
	case oplog.Skipped:
		return theme.JobsOperationSkipped
	}
	return theme.JobsOperationSuccess
}

// IconKey is the row's [icons.jobs.operations] key: its result key, except a skipped row whose
// conflict resolution deleted the source, which has its own glyph.
func (o JobOperation) IconKey() string {
	if o.SourceDeleted {
		return theme.JobsOperationSourceDeleted
	}
	return JobOperationResultKey(o.Result)
}

// JobOperationRow is a row the Operations pane shows: the operation and the label's fuzzy match
// ranges. Pending marks a row whose record is still being read from the spill file.
type JobOperationRow struct {
	Op      JobOperation
	Ranges  []search.Range
	Pending bool
}

// JobsOpsWindow is the part of the selected job's Operations rows the pane draws, prepared by
// the jobs handler each frame (jobs.Handler.RefreshOpsWindow) so rendering never reads the
// operation log or the disk. The pane's display rows are the active item (when Active is set)
// followed by the records, newest first; JobsViewState.OpsScroll counts display rows from that
// newest edge. Rows are the records Offset.. (record rows, not counting the active row); Total
// is the record count of the current view (result filter and fuzzy query applied).
type JobsOpsWindow struct {
	JobID  string
	Offset int
	Total  int
	Rows   []JobOperationRow
	// Scanning is set while a fuzzy query is still being matched against the log.
	Scanning bool
	// Active is the label of the item the job is working on now, the newest display row, drawn
	// with the job icon; empty when the pane shows none (jobs.Handler.RefreshOpsWindow).
	Active string
}

// activeRows is 1 while the active item takes display row 0, else 0.
func (w JobsOpsWindow) activeRows() int {
	if w.Active != "" {
		return 1
	}
	return 0
}

// DisplayTotal is the pane's display row count: the records plus the active row.
func (w JobsOpsWindow) DisplayTotal() int {
	return w.Total + w.activeRows()
}

// RecordSpan maps a viewport of rows display rows starting scroll rows from the newest edge to
// the records it shows: the first record row and how many.
func (w JobsOpsWindow) RecordSpan(scroll, rows int) (offset, count int) {
	act := w.activeRows()
	if scroll < act {
		return 0, max(rows-act, 0)
	}
	return scroll - act, rows
}

// row returns record row i of the window; ok is false when it is outside the prepared rows.
func (w JobsOpsWindow) row(i int) (JobOperationRow, bool) {
	if i < w.Offset || i >= w.Offset+len(w.Rows) {
		return JobOperationRow{}, false
	}
	return w.Rows[i-w.Offset], true
}

// jobOperationIconCols is the icon column width: a one-cell result icon and one space.
const jobOperationIconCols = 2

// JobsOpsMaxScroll is the largest Operations scroll offset for a pane of contentRows rows
// showing total display rows: the pane scrolls by row away from the newest edge.
func JobsOpsMaxScroll(total, contentRows int) int {
	return max(0, total-max(contentRows, 1))
}

// jobOperationLine is one screen line of the Operations pane: an optional one-cell icon and
// the text after the icon column.
type jobOperationLine struct {
	icon      string
	iconStyle tcell.Style
	text      string
	style     tcell.Style
	spans     []primitive.Span
}

// drawJobsOperationsPanel draws the Operations pane from the prepared window: one row per item
// with a result icon, the item in progress first, and the active filters on the bottom border.
// Rows run newest first from the top, or with newestLast oldest first so the newest is the last
// line (a short list stays at the top). wrap word-wraps rows (maximized pane) instead of
// fitting them to one line; scrolling stays by row.
func drawJobsOperationsPanel(screen tcell.Screen, rect Rect, state JobsViewState, win JobsOpsWindow, sourceLabel string, styles theme.Theme, chromeBlocked, focused, wrap, newestLast bool) {
	f := state.OpsFilter
	queryOpen := f.Query != "" || f.Editing
	title := " Operations "
	if queryOpen {
		title = "" // the fuzzy query owns the top row
	}
	endLabel := ""
	if sourceLabel != "" {
		// Rows filtered to one source; at most half the border so the title keeps its room.
		endLabel = panelSelectionSizePadded(primitive.FitPathForWidth(sourceLabel, max(rect.Width/2-2, 1)))
	}
	layout := drawAuxPanelChrome(screen, rect, title, endLabel, focused, chromeBlocked, false, styles)
	if queryOpen {
		input := styles.FuzzyInput
		switch {
		case chromeBlocked:
			input = styles.PanelBottomIndicator(theme.PanelBottomIndicatorKeySelectionSize, focused, chromeBlocked)
		case f.Query != "" && win.Total == 0 && !win.Scanning:
			input = styles.FuzzyInputNomatch
		}
		drawFuzzyQueryTitle(screen, layout.TitleX, rect.Y, layout.TitleWidth, f.Query, f.Cursor, f.Editing && focused && !chromeBlocked, input)
	}
	drawJobsOperationsFilterLabel(screen, rect, state, win, styles, chromeBlocked, focused)

	contentTop := rect.Y + 1
	contentH := rect.Height - 2
	if contentH <= 0 {
		return
	}
	body := auxPanelBodyText(styles, chromeBlocked, layout.ContentBG)
	textX := rect.X + 2
	textW := max(rect.Width-4, 1)
	if win.DisplayTotal() == 0 {
		if contentH > 1 && !win.Scanning {
			primitive.Text(screen, textX, contentTop+1, textW, styles.EmptyText(), styles.PanelEmptyStyle().Background(layout.ContentBG))
		}
		return
	}
	lineW := max(textW-jobOperationIconCols, 1)
	matchStyle := styles.FuzzyHighlight.Background(layout.ContentBG)
	pending := styles.PanelEmptyStyle().Background(layout.ContentBG)

	// Collect whole rows from the newest visible one until the pane is full.
	var groups [][]jobOperationLine
	lines := 0
	for d := min(max(state.OpsScroll, 0), JobsOpsMaxScroll(win.DisplayTotal(), contentH)); d < win.DisplayTotal() && lines < contentH; d++ {
		var g []jobOperationLine
		if d < win.activeRows() {
			g = []jobOperationLine{jobOperationActiveLine(win.Active, lineW, styles, body, chromeBlocked)}
		} else if r, ok := win.row(d - win.activeRows()); !ok || r.Pending {
			g = []jobOperationLine{{text: "…", style: pending}}
		} else {
			g = jobOperationRowLines(r, lineW, wrap, styles, body, matchStyle, chromeBlocked)
		}
		groups = append(groups, g)
		lines += len(g)
	}
	if newestLast {
		slices.Reverse(groups)
	}
	var out []jobOperationLine
	for _, g := range groups {
		out = append(out, g...)
	}
	if len(out) > contentH {
		if newestLast {
			out = out[len(out)-contentH:] // keep the newest row whole at the bottom
		} else {
			out = out[:contentH]
		}
	}
	for i, l := range out {
		y := contentTop + i
		if l.icon != "" {
			primitive.Text(screen, textX, y, 1, l.icon, l.iconStyle)
		}
		primitive.StyledText(screen, textX+jobOperationIconCols, y, lineW, l.text, l.style, l.spans)
	}
}

// jobOperationActiveLine is the row of the item the job is working on: the running status icon
// (icons.jobs.list.running, as in the queue) and its path-fitted label.
func jobOperationActiveLine(label string, lineW int, styles theme.Theme, body tcell.Style, chromeBlocked bool) jobOperationLine {
	iconStyle := body
	if !chromeBlocked {
		iconStyle = styles.JobsIconStyleOn("running", body)
	}
	return jobOperationLine{icon: styles.IconJobsList("running"), iconStyle: iconStyle, text: primitive.FitPathForWidth(label, lineW), style: body}
}

// jobOperationRowLines is a record row's lines: one fitted line, or with wrap its word-wrapped
// text, the result icon on the first.
func jobOperationRowLines(r JobOperationRow, lineW int, wrap bool, styles theme.Theme, body, matchStyle tcell.Style, chromeBlocked bool) []jobOperationLine {
	resultStyle := body
	if !chromeBlocked {
		resultStyle = styles.JobsOperationOn(JobOperationResultKey(r.Op.Result), body)
	}
	first := jobOperationLine{icon: styles.IconJobsOperation(r.Op.IconKey()), iconStyle: resultStyle, style: body}
	if !wrap {
		text, labelEnd, labelWhole := fitJobOperation(r.Op, lineW)
		first.text = text
		if !chromeBlocked {
			first.spans = jobOperationSpans(r, labelEnd, labelWhole, text, matchStyle, resultStyle)
		}
		return []jobOperationLine{first}
	}
	var out []jobOperationLine
	for li, part := range WrapTextLines(r.Op.Text(), lineW) {
		l := jobOperationLine{text: part, style: body}
		if li == 0 {
			l = first
			l.text = part
			if part == r.Op.Text() && !chromeBlocked {
				l.spans = jobOperationSpans(r, utf8.RuneCountInString(r.Op.Label), true, part, matchStyle, resultStyle)
			}
		}
		out = append(out, l)
	}
	if len(out) == 0 {
		out = append(out, first)
	}
	return out
}

// jobOperationSpans highlights a row's fuzzy matches (only on an unshortened label) and colors
// its reason in the result style.
func jobOperationSpans(r JobOperationRow, labelEnd int, labelWhole bool, text string, matchStyle, resultStyle tcell.Style) []primitive.Span {
	var spans []primitive.Span
	if labelWhole {
		spans = shiftSpans(r.Ranges, 0, matchStyle)
	}
	if r.Op.Reason != "" {
		spans = append(spans, primitive.Span{Start: labelEnd, End: utf8.RuneCountInString(text), Style: resultStyle})
	}
	return spans
}

// fitJobOperation fits op's one-line text to w columns: the label is path-fitted (keeping the
// basename) and a ": <reason>" suffix is truncated on the right, the label keeping at least
// half the width. labelEnd is the label's rune length in text; labelWhole reports it unshortened.
func fitJobOperation(op JobOperation, w int) (text string, labelEnd int, labelWhole bool) {
	full := op.Text()
	labelRunes := utf8.RuneCountInString(op.Label)
	if utf8.RuneCountInString(full) <= w {
		return full, labelRunes, true
	}
	if full == op.Label {
		fit := primitive.FitPathForWidth(op.Label, w)
		return fit, utf8.RuneCountInString(fit), fit == op.Label
	}
	reasonPart := full[len(op.Label):]
	budget := max(w-utf8.RuneCountInString(reasonPart), min(labelRunes, w/2))
	fit := primitive.FitPathForWidth(op.Label, budget)
	fitRunes := utf8.RuneCountInString(fit)
	return fit + primitive.TruncateRight(reasonPart, max(w-fitRunes, 0)), fitRunes, fit == op.Label
}

// drawJobsOperationsFilterLabel paints the result filter (unless All) and "filtering…" while
// the fuzzy query is still being matched centered on the bottom border, joined by " · ".
func drawJobsOperationsFilterLabel(screen tcell.Screen, rect Rect, state JobsViewState, win JobsOpsWindow, styles theme.Theme, chromeBlocked, focused bool) {
	indicator := styles.PanelBottomIndicator(theme.PanelBottomIndicatorKeySelectionSize, focused, chromeBlocked)
	var segs []styledSegment
	if shown := state.OpsResultShown(); shown != dialog.JobResultAll {
		segs = append(segs, styledSegment{Text: shown.Label(), Style: indicator})
	}
	if win.Scanning {
		if len(segs) > 0 {
			segs = append(segs, styledSegment{Text: " · ", Style: indicator})
		}
		segs = append(segs, styledSegment{Text: "filtering…", Style: indicator})
	}
	if len(segs) == 0 {
		return
	}
	drawAuxPanelBottomCenterSpans(screen, rect, segs)
}
