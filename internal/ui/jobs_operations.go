package ui

import (
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// jobOperationsMaxRows caps a job's Operations log to avoid unbounded memory in long sessions.
const jobOperationsMaxRows = 200

// JobOperation is one row of the jobs view Operations pane: a file the job processed and its
// result. Error is the failure text of a failed row.
type JobOperation struct {
	Label  string
	Error  string
	Failed bool
}

// Text is the row's displayed text: the label, plus ": <error>" on a failed row.
func (o JobOperation) Text() string {
	if o.Failed && o.Error != "" {
		return o.Label + ": " + o.Error
	}
	return o.Label
}

// AppendJobOperation records op in a job's Operations log (oldest first). A success repeating
// the last success is dropped (progress samples name the same file several times) and replaces
// an earlier failure of the same file (a retry); a failure replaces every earlier row of its file.
func AppendJobOperation(ops []JobOperation, op JobOperation) []JobOperation {
	if n := len(ops); !op.Failed && n > 0 && !ops[n-1].Failed && ops[n-1].Label == op.Label {
		return ops
	}
	ops = slices.DeleteFunc(ops, func(o JobOperation) bool {
		return o.Label == op.Label && (op.Failed || o.Failed)
	})
	return capJobOperations(append(ops, op))
}

// capJobOperations trims ops to jobOperationsMaxRows, dropping the oldest successes first and
// failures only when nothing else is left.
func capJobOperations(ops []JobOperation) []JobOperation {
	excess := len(ops) - jobOperationsMaxRows
	if excess <= 0 {
		return ops
	}
	out := make([]JobOperation, 0, jobOperationsMaxRows)
	for _, o := range ops {
		if excess > 0 && !o.Failed {
			excess--
			continue
		}
		out = append(out, o)
	}
	return out[excess:]
}

// CountFailedJobOperations is the number of failed rows in ops (the top-border failure count).
func CountFailedJobOperations(ops []JobOperation) int {
	n := 0
	for _, o := range ops {
		if o.Failed {
			n++
		}
	}
	return n
}

// JobOperationRow is a row the Operations pane shows: the operation and the label's fuzzy
// match ranges.
type JobOperationRow struct {
	Op     JobOperation
	Ranges []search.Range
}

// VisibleJobOperations returns the rows the Operations pane shows, newest first: those whose
// result passes the view's result filter and whose label fuzzy-matches its query. It is the
// one row source for rendering, scroll bounds and the maximized pane.
func VisibleJobOperations(ops []JobOperation, state JobsViewState, caseInsensitive bool) []JobOperationRow {
	shown := state.OpsResultShown()
	query := search.Parse(state.OpsFilter.Query)
	opts := search.Options{CaseInsensitive: caseInsensitive}
	rows := make([]JobOperationRow, 0, len(ops))
	for i := len(ops) - 1; i >= 0; i-- {
		o := ops[i]
		if !shown.Shows(o.Failed) {
			continue
		}
		var ranges []search.Range
		if !query.Empty() {
			r := query.Match(o.Label, opts)
			if !r.Matched {
				continue
			}
			ranges = r.Ranges
		}
		rows = append(rows, JobOperationRow{Op: o, Ranges: ranges})
	}
	return rows
}

// jobOperationLine is one painted line of the Operations pane. First marks a row's first line
// (it carries the icon); Spans are only set on a row shown whole on one line.
type jobOperationLine struct {
	Text   string
	Failed bool
	First  bool
	Row    JobOperationRow
	Whole  bool
}

// jobOperationIconCols is the icon column width: a one-cell result icon and one space.
const jobOperationIconCols = 2

// jobOperationLines lays rows out in a pane of text width textW. Unwrapped rows are one line
// each (paths fitted at paint time); wrapped rows break at textW and indent continuations under
// the text.
func jobOperationLines(rows []JobOperationRow, textW int, wrap bool) []jobOperationLine {
	out := make([]jobOperationLine, 0, len(rows))
	for _, r := range rows {
		text := r.Op.Text()
		if !wrap {
			out = append(out, jobOperationLine{Text: text, Failed: r.Op.Failed, First: true, Row: r, Whole: true})
			continue
		}
		parts := WrapTextLines(text, max(textW, 1))
		for i, p := range parts {
			out = append(out, jobOperationLine{Text: p, Failed: r.Op.Failed, First: i == 0, Row: r, Whole: len(parts) == 1 && p == text})
		}
	}
	return out
}

// JobOperationsLineCount is the number of scrollable lines the Operations pane drawn in rect
// paints for rows (at least one, like the other jobs panes); wrap matches the maximized pane.
func JobOperationsLineCount(rows []JobOperationRow, rect Rect, wrap bool) int {
	return max(len(jobOperationLines(rows, max(rect.Width-4-jobOperationIconCols, 1), wrap)), 1)
}

// fitJobOperation fits op's one-line text to w columns: the label is path-fitted (keeping the
// basename) and a failed row's ": <error>" is truncated on the right, the label keeping at least
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
	errPart := full[len(op.Label):]
	budget := max(w-utf8.RuneCountInString(errPart), min(labelRunes, w/2))
	fit := primitive.FitPathForWidth(op.Label, budget)
	fitRunes := utf8.RuneCountInString(fit)
	return fit + primitive.TruncateRight(errPart, max(w-fitRunes, 0)), fitRunes, fit == op.Label
}

// jobOperationsFailureLabel is the top-border failure count, empty without failures.
func jobOperationsFailureLabel(failed int) string {
	switch failed {
	case 0:
		return ""
	case 1:
		return "1 failure"
	}
	return fmt.Sprintf("%d failures", failed)
}

// drawJobsOperationsPanel draws the Operations pane: one row per processed file, newest first,
// with a result icon, the failure count on the top border and the active filters on the bottom
// border. wrap word-wraps rows (maximized pane) instead of fitting paths to one line.
func drawJobsOperationsPanel(screen tcell.Screen, rect Rect, state JobsViewState, ops []JobOperation, caseInsensitive bool, styles theme.Theme, chromeBlocked, focused, wrap bool) {
	endLabel := panelSelectionSizePadded(jobOperationsFailureLabel(CountFailedJobOperations(ops)))
	failedOn := func(base tcell.Style) tcell.Style { return styles.JobsOperationOn(true, base) }
	layout := drawAuxPanelChromeEndStyled(screen, rect, " Operations ", endLabel, failedOn, focused, chromeBlocked, false, styles)
	rows := VisibleJobOperations(ops, state, caseInsensitive)
	drawJobsOperationsFilterLabel(screen, rect, state, len(rows) > 0, styles, chromeBlocked, focused)

	contentTop := rect.Y + 1
	contentH := rect.Height - 2
	if contentH <= 0 {
		return
	}
	body := auxPanelBodyText(styles, chromeBlocked, layout.ContentBG)
	textX := rect.X + 2
	textW := max(rect.Width-4, 1)
	if len(rows) == 0 {
		if contentH > 1 {
			primitive.Text(screen, textX, contentTop+1, textW, styles.EmptyText(), styles.PanelEmptyStyle().Background(layout.ContentBG))
		}
		return
	}
	iconW := jobOperationIconCols
	lineW := max(textW-iconW, 1)
	lines := jobOperationLines(rows, lineW, wrap)
	scroll := min(max(state.OpsScroll, 0), max(0, len(lines)-contentH))
	matchStyle := styles.FuzzyHighlight.Background(layout.ContentBG)
	for i := 0; i < contentH && i+scroll < len(lines); i++ {
		l := lines[i+scroll]
		y := contentTop + i
		resultStyle := body
		if !chromeBlocked {
			resultStyle = styles.JobsOperationOn(l.Failed, body)
		}
		if l.First {
			primitive.Text(screen, textX, y, 1, styles.IconJobsOperation(l.Failed), resultStyle)
		}
		text, labelEnd, labelWhole := l.Text, utf8.RuneCountInString(l.Row.Op.Label), l.Whole
		if !wrap {
			text, labelEnd, labelWhole = fitJobOperation(l.Row.Op, lineW)
		}
		var spans []primitive.Span
		if l.Whole && !chromeBlocked {
			if labelWhole {
				spans = shiftSpans(l.Row.Ranges, 0, matchStyle)
			}
			if l.Row.Op.Failed && l.Row.Op.Error != "" {
				spans = append(spans, primitive.Span{Start: labelEnd, End: utf8.RuneCountInString(text), Style: resultStyle})
			}
		}
		primitive.StyledText(screen, textX+iconW, y, lineW, text, body, spans)
	}
}

// drawJobsOperationsFilterLabel paints the active Operations filters centered on the bottom
// border: the result filter (unless All), then the fuzzy query as "> query" (with a caret while
// editing in the focused pane), joined by " · ".
func drawJobsOperationsFilterLabel(screen tcell.Screen, rect Rect, state JobsViewState, hasRows bool, styles theme.Theme, chromeBlocked, focused bool) {
	indicator := styles.PanelBottomIndicator(theme.PanelBottomIndicatorKeySelectionSize, focused, chromeBlocked)
	var segs []styledSegment
	if shown := state.OpsResultShown(); shown != dialog.JobResultAll {
		segs = append(segs, styledSegment{Text: shown.Label(), Style: indicator})
	}
	f := state.OpsFilter
	if f.Query != "" || f.Editing {
		if len(segs) > 0 {
			segs = append(segs, styledSegment{Text: " · ", Style: indicator})
		}
		input := styles.FuzzyInput
		if f.Query != "" && !hasRows {
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
			at := " "
			rest := ""
			if cur < len(runes) {
				at, rest = string(runes[cur]), string(runes[cur+1:])
			}
			segs = append(segs,
				styledSegment{Text: "> " + string(runes[:cur]), Style: input},
				styledSegment{Text: at, Style: input.Reverse(true)},
				styledSegment{Text: rest, Style: input})
		}
	}
	if len(segs) == 0 {
		return
	}
	drawAuxPanelBottomCenterSpans(screen, rect, segs)
}
