package dialog

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// drawHelpDialog renders the centered help dialog with fuzzy shortcut search.
func DrawHelpDialog(screen tcell.Screen, layout Layout, state HelpViewState, styles theme.Theme) {
	metrics, ok := ComputeHelpDialogListMetrics(layout)
	if !ok {
		return
	}
	rect := metrics.Rect
	title := state.Title
	if title == "" {
		title = "Help"
	}
	borderStyle := draw.DrawDialogFrame(screen, rect, title, styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	itemBg := dbg
	primaryCol := rect.X + 2
	inputWidth := metrics.InputWidth
	listH := metrics.ListH

	filterFocused := state.Focus == 0
	draw.DrawScrollingDialogInput(screen, primaryCol, rect.Y+1, inputWidth, draw.ScrollingInputState{Value: state.Query, Cursor: state.QueryCursor, Scroll: state.QueryScroll, LeadingSymbol: styles.SymbolSearchIcon()}, filterFocused, false, styles)

	// Separator before list.
	listTop := rect.Y + 2
	draw.DrawDialogHSeparator(screen, rect, listTop, borderStyle)

	// List rows start right after the separator — no column-title row. Rows are always grouped
	// under non-selectable section headers, in browse mode and while filtering alike.
	rowWidth := inputWidth
	visualRows := BuildHelpVisualRows(state.Entries, state.Ranked)
	for row := 0; row < listH; row++ {
		y := listTop + 1 + row
		if y >= rect.Y+rect.Height-2 {
			break
		}
		visualIdx := state.ListScroll + row
		baseStyle := styles.DialogText.Background(itemBg)
		line := ""
		keyStart := -1
		keyEnd := -1
		var ranges []search.Range
		isCursor := false
		isHeader := false
		if visualIdx >= 0 && visualIdx < len(visualRows) {
			vr := visualRows[visualIdx]
			if vr.IsHeader {
				isHeader = true
				line = vr.Header
			} else if vr.RankedIdx < len(state.Ranked) {
				entIdx := state.Ranked[vr.RankedIdx]
				if entIdx >= 0 && entIdx < len(state.Entries) {
					ent := state.Entries[entIdx]
					line, keyStart, keyEnd = FormatHelpRow(ent, metrics.KeyColWidth, rowWidth)
					if entIdx < len(state.MatchRanges) {
						ranges = state.MatchRanges[entIdx]
					}
				}
				isCursor = state.Focus == 0 && vr.RankedIdx == state.Selected
			}
		}
		matchStyle := styles.FuzzyHighlight
		switch {
		case isHeader:
			baseStyle = styles.DialogHelpSection.Background(itemBg)
		case isCursor:
			baseStyle = styles.DialogOptionRowStyle(true, false)
			matchStyle = styles.FuzzyHighlightCursor
		}
		_, rowBg, _ := baseStyle.Decompose()
		matchStyle = matchStyle.Background(rowBg)

		rendered, spans := helpRowContent(line, ranges, rowWidth, matchStyle)
		if keyStart >= 0 && !isCursor {
			spans = append(spans, primitive.Span{Start: keyStart, End: keyEnd, Style: styles.DialogHelpKey.Background(rowBg)})
		}
		primitive.StyledText(screen, primaryCol, y, rowWidth, rendered, baseStyle, spans)
	}

	// Separator after list.
	sepAfterList := listTop + 1 + listH
	draw.DrawDialogHSeparator(screen, rect, sepAfterList, borderStyle)

	// Button row: single Close button.
	buttonY := rect.Y + rect.Height - 2
	closeFocused := state.Focus == 1
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "Close", Shortcut: 'C', Focused: closeFocused},
	}, styles)
}

// FormatHelpRow builds a single text line for a help entry: the shortcut keys right-aligned
// within a fixed-width column on the left, then a guaranteed one-space gap, then the title.
// Keys wider than keyColWidth still get their one-space gap (the column grows for that row).
// keyStart/keyEnd is the rune range where the key text sits, for styling it distinctly from
// the rest of the row.
func FormatHelpRow(ent HelpEntry, keyColWidth, width int) (line string, keyStart, keyEnd int) {
	if width <= 0 {
		return "", 0, 0
	}
	keys := []rune(ent.Keys)
	pad := keyColWidth - len(keys)
	if pad < 0 {
		pad = 0
	}
	keyStart, keyEnd = pad, pad+len(keys)
	line = strings.Repeat(" ", pad) + string(keys) + " " + ent.Title
	if r := []rune(line); len(r) > width {
		line = string(r[:width])
	}
	return line, keyStart, keyEnd
}

// HelpVisualRow is one on-screen row of the help list: a non-selectable section-header row
// (IsHeader true, Header set) or an entry row (IsHeader false, RankedIdx indexes into
// Ranked/Entries).
type HelpVisualRow struct {
	IsHeader  bool
	Header    string
	RankedIdx int
}

// BuildHelpVisualRows expands Ranked into on-screen rows, inserting a section-header row
// before each run of same-section entries, in Ranked order — in browse mode (empty filter
// query) that's section-contiguous because Entries itself was built in section-priority order
// (see App.buildHelpEntriesForView); while filtering, Ranked is relevance-ordered instead, so
// a header can repeat if matches from the same section aren't adjacent in the ranked results.
// Entries follow their header immediately, with no spacer row between sections — the dialog's
// separator above the list already provides that break for the first header, and repeating it
// for every section would waste a row of vertical space.
func BuildHelpVisualRows(entries []HelpEntry, ranked []int) []HelpVisualRow {
	rows := make([]HelpVisualRow, 0, len(ranked))
	prevSection := ""
	havePrev := false
	for i, entIdx := range ranked {
		if entIdx < 0 || entIdx >= len(entries) {
			continue
		}
		section := entries[entIdx].Section
		if !havePrev || section != prevSection {
			rows = append(rows, HelpVisualRow{IsHeader: true, Header: section})
		}
		rows = append(rows, HelpVisualRow{RankedIdx: i})
		prevSection = section
		havePrev = true
	}
	return rows
}

func helpRowContent(line string, ranges []search.Range, width int, matchStyle tcell.Style) (string, []primitive.Span) {
	if width <= 0 {
		return "", nil
	}
	orig := []rune(line)
	var disp []rune
	switch {
	case len(orig) <= width:
		disp = orig
	case width == 1:
		disp = orig[:1]
	default:
		disp = append(append([]rune{}, orig[:width-1]...), primitive.Ellipsis)
	}
	spans := make([]primitive.Span, 0, len(ranges))
	truncated := len(orig) > width
	for i := range disp {
		if truncated && i == len(disp)-1 && disp[i] == primitive.Ellipsis {
			continue
		}
		if helpRangeContains(ranges, i) {
			spans = append(spans, primitive.Span{Start: i, End: i + 1, Style: matchStyle})
		}
	}
	return string(disp), spans
}

func helpRangeContains(ranges []search.Range, index int) bool {
	for _, r := range ranges {
		if index >= r.Start && index < r.End {
			return true
		}
	}
	return false
}
