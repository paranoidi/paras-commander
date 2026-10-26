package ui

import (
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
	Result oplog.Result
	Label  string
	Reason string
}

// JobOperationFromRecord converts an operation log record to a pane row.
func JobOperationFromRecord(r oplog.Record) JobOperation {
	return JobOperation{Result: r.Result, Label: r.Label, Reason: r.Reason}
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

// JobOperationRow is a row the Operations pane shows: the operation and the label's fuzzy match
// ranges. Pending marks a row whose record is still being read from the spill file.
type JobOperationRow struct {
	Op      JobOperation
	Ranges  []search.Range
	Pending bool
}

// JobsOpsWindow is the part of the selected job's Operations rows the pane draws, prepared by
// the jobs handler each frame (jobs.Handler.RefreshOpsWindow) so rendering never reads the
// operation log or the disk. Rows are display rows Offset.., newest first; Total is the row
// count of the current view (result filter and fuzzy query applied).
type JobsOpsWindow struct {
	JobID  string
	Offset int
	Total  int
	Rows   []JobOperationRow
	// Scanning is set while a fuzzy query is still being matched against the log.
	Scanning bool
}

// row returns display row i of the window; ok is false when it is outside the prepared rows.
func (w JobsOpsWindow) row(i int) (JobOperationRow, bool) {
	if i < w.Offset || i >= w.Offset+len(w.Rows) {
		return JobOperationRow{}, false
	}
	return w.Rows[i-w.Offset], true
}

// jobOperationIconCols is the icon column width: a one-cell result icon and one space.
const jobOperationIconCols = 2

// JobsOpsMaxScroll is the largest Operations scroll offset for a pane of contentRows rows:
// the pane scrolls by row, newest at the top.
func JobsOpsMaxScroll(total, contentRows int) int {
	return max(0, total-max(contentRows, 1))
}

// drawJobsOperationsPanel draws the Operations pane from the prepared window: one row per item,
// newest first, with a result icon and the active filters on the bottom border. wrap word-wraps
// rows (maximized pane) instead of fitting them to one line; scrolling stays by row.
func drawJobsOperationsPanel(screen tcell.Screen, rect Rect, state JobsViewState, win JobsOpsWindow, styles theme.Theme, chromeBlocked, focused, wrap bool) {
	layout := drawAuxPanelChrome(screen, rect, " Operations ", "", focused, chromeBlocked, false, styles)
	drawJobsOperationsFilterLabel(screen, rect, state, win, styles, chromeBlocked, focused)

	contentTop := rect.Y + 1
	contentH := rect.Height - 2
	if contentH <= 0 {
		return
	}
	body := auxPanelBodyText(styles, chromeBlocked, layout.ContentBG)
	textX := rect.X + 2
	textW := max(rect.Width-4, 1)
	if win.Total == 0 {
		if contentH > 1 && !win.Scanning {
			primitive.Text(screen, textX, contentTop+1, textW, styles.EmptyText(), styles.PanelEmptyStyle().Background(layout.ContentBG))
		}
		return
	}
	lineW := max(textW-jobOperationIconCols, 1)
	matchStyle := styles.FuzzyHighlight.Background(layout.ContentBG)
	y := contentTop
	for i := min(max(state.OpsScroll, 0), JobsOpsMaxScroll(win.Total, contentH)); i < win.Total && y < contentTop+contentH; i++ {
		r, ok := win.row(i)
		if !ok || r.Pending {
			primitive.Text(screen, textX+jobOperationIconCols, y, lineW, "…", styles.PanelEmptyStyle().Background(layout.ContentBG))
			y++
			continue
		}
		key := JobOperationResultKey(r.Op.Result)
		resultStyle := body
		if !chromeBlocked {
			resultStyle = styles.JobsOperationOn(key, body)
		}
		primitive.Text(screen, textX, y, 1, styles.IconJobsOperation(key), resultStyle)
		if wrap {
			for li, part := range WrapTextLines(r.Op.Text(), lineW) {
				if y >= contentTop+contentH {
					break
				}
				var spans []primitive.Span
				if li == 0 && part == r.Op.Text() && !chromeBlocked {
					spans = jobOperationSpans(r, utf8.RuneCountInString(r.Op.Label), true, part, matchStyle, resultStyle)
				}
				primitive.StyledText(screen, textX+jobOperationIconCols, y, lineW, part, body, spans)
				y++
			}
			continue
		}
		text, labelEnd, labelWhole := fitJobOperation(r.Op, lineW)
		var spans []primitive.Span
		if !chromeBlocked {
			spans = jobOperationSpans(r, labelEnd, labelWhole, text, matchStyle, resultStyle)
		}
		primitive.StyledText(screen, textX+jobOperationIconCols, y, lineW, text, body, spans)
		y++
	}
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

// drawJobsOperationsFilterLabel paints the active Operations filters centered on the bottom
// border: the result filter (unless All), then the fuzzy query as "> query" (with a caret while
// editing in the focused pane), then "filtering…" while the query is still being matched, all
// joined by " · ".
func drawJobsOperationsFilterLabel(screen tcell.Screen, rect Rect, state JobsViewState, win JobsOpsWindow, styles theme.Theme, chromeBlocked, focused bool) {
	indicator := styles.PanelBottomIndicator(theme.PanelBottomIndicatorKeySelectionSize, focused, chromeBlocked)
	sep := styledSegment{Text: " · ", Style: indicator}
	var segs []styledSegment
	if shown := state.OpsResultShown(); shown != dialog.JobResultAll {
		segs = append(segs, styledSegment{Text: shown.Label(), Style: indicator})
	}
	f := state.OpsFilter
	if f.Query != "" || f.Editing {
		if len(segs) > 0 {
			segs = append(segs, sep)
		}
		input := styles.FuzzyInput
		if f.Query != "" && win.Total == 0 && !win.Scanning {
			input = styles.FuzzyInputNomatch
		}
		if chromeBlocked {
			input = indicator
		}
		runes := []rune(f.Query)
		if !f.Editing || !focused || chromeBlocked {
			segs = append(segs, styledSegment{Text: "> " + f.Query, Style: input})
		} else {
			cur := min(max(f.Cursor, 0), len(runes))
			at, rest := " ", ""
			if cur < len(runes) {
				at, rest = string(runes[cur]), string(runes[cur+1:])
			}
			segs = append(segs,
				styledSegment{Text: "> " + string(runes[:cur]), Style: input},
				styledSegment{Text: at, Style: input.Reverse(true)},
				styledSegment{Text: rest, Style: input})
		}
	}
	if win.Scanning {
		if len(segs) > 0 {
			segs = append(segs, sep)
		}
		segs = append(segs, styledSegment{Text: "filtering…", Style: indicator})
	}
	if len(segs) == 0 {
		return
	}
	drawAuxPanelBottomCenterSpans(screen, rect, segs)
}
