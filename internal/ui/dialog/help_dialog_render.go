package dialog

import (
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

	// Filter label.
	primitive.Text(screen, primaryCol, rect.Y+1, inputWidth, "Filter:", styles.DialogText.Background(itemBg))

	filterFocused := state.Focus == 0
	draw.DrawScrollingDialogInput(screen, primaryCol, rect.Y+3, inputWidth, draw.ScrollingInputState{Value: state.Query, Cursor: state.QueryCursor, Scroll: state.QueryScroll}, filterFocused, false, styles)

	// Separator before list.
	sepBeforeList := rect.Y + 4
	draw.DrawDialogHSeparator(screen, rect, sepBeforeList, borderStyle)

	// List header.
	listTop := rect.Y + 5
	headerStyle := styles.DialogText.Background(itemBg)
	headerLine := padRight("Key", metrics.KeyPad) + padRight("Section", metrics.SecPad) + "Action"
	if n := len([]rune(headerLine)); n > inputWidth {
		headerLine = string([]rune(headerLine)[:inputWidth])
	}
	primitive.Text(screen, primaryCol, listTop, inputWidth, headerLine, headerStyle)

	// List rows.
	rowWidth := inputWidth
	for row := 0; row < listH; row++ {
		y := listTop + 1 + row
		if y >= rect.Y+rect.Height-2 {
			break
		}
		idxInRank := state.ListScroll + row
		baseStyle := styles.DialogText.Background(itemBg)
		line := ""
		var ranges []search.Range
		isCursor := false
		if idxInRank < len(state.Ranked) {
			entIdx := state.Ranked[idxInRank]
			if entIdx >= 0 && entIdx < len(state.Entries) {
				ent := state.Entries[entIdx]
				line = FormatHelpRow(ent, 0, metrics.KeyPad, metrics.KeyPad+metrics.SecPad, rowWidth)
				if entIdx < len(state.MatchRanges) {
					ranges = state.MatchRanges[entIdx]
				}
			}
			isCursor = state.Focus == 0 && idxInRank == state.Selected
		}
		matchStyle := styles.FuzzyHighlight
		if isCursor {
			baseStyle = styles.DialogOptionRowStyle(true, false)
			matchStyle = styles.FuzzyHighlightCursor
		}
		_, rowBg, _ := baseStyle.Decompose()
		matchStyle = matchStyle.Background(rowBg)

		rendered, spans := helpRowContent(line, ranges, rowWidth, matchStyle)
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

// FormatHelpRow builds a single text line for a help entry (padded columns then title).
// colKey/colSection/colTitle are absolute column indices only for computing pad widths:
// use colKey=0, colSection=keyPad, colTitle=keyPad+secPad when callers have pad widths.
func FormatHelpRow(ent HelpEntry, colKey, colSection, colTitle, width int) string {
	keys := ent.Keys
	section := ent.Section
	row := padRight(keys, colSection-colKey)
	row += padRight(section, colTitle-colSection)
	row += ent.Title
	if n := len([]rune(row)); n > width {
		row = string([]rune(row)[:width])
	}
	return row
}

func padRight(s string, minWidth int) string {
	r := []rune(s)
	if len(r) >= minWidth {
		return s + " "
	}
	return s + string(make([]rune, minWidth-len(r)))
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
		disp = append(append([]rune{}, orig[:width-1]...), '~')
	}
	spans := make([]primitive.Span, 0, len(ranges))
	truncated := len(orig) > width
	for i := range disp {
		if truncated && i == len(disp)-1 && disp[i] == '~' {
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
