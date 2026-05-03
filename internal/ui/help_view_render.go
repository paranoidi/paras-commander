package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// drawHelpDialog renders the centered help dialog with fuzzy shortcut search.
func drawHelpDialog(screen tcell.Screen, layout Layout, state HelpViewState, styles theme.Theme) {
	// Compute dialog size with 7-char margins left/right and ~7 rows margin top/bottom.
	maxW := layout.Width - 14
	if maxW < 40 {
		maxW = 40
	}
	if maxW > 90 {
		maxW = 90
	}
	maxH := layout.Height - 14
	if maxH < 12 {
		maxH = 12
	}
	if maxH > 36 {
		maxH = 36
	}

	// Chrome: title(1) + sep(1) + filter label(1) + blank(1) + filter input(1) + sep(1) + header(1) + button row(2) = 9
	listH := maxH - 9
	if listH < 4 {
		listH = 4
	}
	height := 9 + listH
	if height > layout.Height-2 {
		height = layout.Height - 2
		listH = height - 9
		if listH < 4 {
			return
		}
	}

	rect := centeredDialogRect(layout, maxW, height)
	borderStyle := drawDialogFrame(screen, rect, "Help", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	itemBg := dbg
	leftCol := rect.X + 2
	inputWidth := rect.Width - 4
	if inputWidth < 10 {
		return
	}

	// Filter label.
	primitive.Text(screen, leftCol, rect.Y+1, inputWidth, "Filter:", styles.DialogText.Background(itemBg))

	filterFocused := state.Focus == 0
	drawSimpleDialogInput(screen, leftCol, rect.Y+3, inputWidth, state.Query, filterFocused, styles)

	// Separator before list.
	sepBeforeList := rect.Y + 4
	drawDialogHSeparator(screen, rect, sepBeforeList, borderStyle)

	// List header.
	listTop := rect.Y + 5
	headerStyle := styles.DialogText.Background(itemBg)
	colKey := leftCol
	colSection := leftCol + 28
	if colSection > rect.X+rect.Width-3 {
		colSection = rect.X + rect.Width - 3
	}
	colTitle := leftCol + 50
	if colTitle > rect.X+rect.Width-3 {
		colTitle = rect.X + rect.Width - 3
	}
	headerLine := padRight("Key", colSection-colKey) + padRight("Section", colTitle-colSection) + "Action"
	if n := len([]rune(headerLine)); n > inputWidth {
		headerLine = string([]rune(headerLine)[:inputWidth])
	}
	primitive.Text(screen, leftCol, listTop, inputWidth, headerLine, headerStyle)

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
				line = formatHelpRow(ent, colKey, colSection, colTitle, rowWidth)
				if entIdx < len(state.MatchRanges) {
					ranges = state.MatchRanges[entIdx]
				}
			}
			isCursor = state.Focus == 0 && idxInRank == state.Selected
		}
		matchStyle := styles.FuzzyHighlight
		if isCursor {
			baseStyle = styles.DialogOptionActive
			matchStyle = styles.FuzzyHighlightCursor
		}
		_, rowBg, _ := baseStyle.Decompose()
		matchStyle = matchStyle.Background(rowBg)

		rendered, spans := helpRowContent(line, ranges, rowWidth, matchStyle)
		primitive.StyledText(screen, leftCol, y, rowWidth, rendered, baseStyle, spans)
	}

	// Separator after list.
	sepAfterList := listTop + 1 + listH
	drawDialogHSeparator(screen, rect, sepAfterList, borderStyle)

	// Button row: single Close button.
	buttonY := rect.Y + rect.Height - 2
	closeFocused := state.Focus == 1
	drawDialogButtonRowCentered(screen, rect, buttonY, []DialogButtonSpec{
		{Label: "Close", Shortcut: 'C', Focused: closeFocused},
	}, styles)
}

// formatHelpRow builds a single text line for a help entry.
func formatHelpRow(ent HelpEntry, colKey, colSection, colTitle, width int) string {
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
