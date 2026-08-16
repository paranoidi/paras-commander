package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// runForEachHistoryPickerDialogHeight returns the outer dialog height for the run-for-each
// command-history picker. Mirrors massRenamePatternPickerDialogHeight
// (mass_rename_pattern_dialog_render.go) — both size a query row + fuzzy list picker the same
// way; a future cleanup could share one implementation if a third picker needs this shape.
func runForEachHistoryPickerDialogHeight(layoutHeight int) int {
	listH := layoutHeight - 12
	switch {
	case listH > 18:
		listH = 18
	case listH < 4:
		listH = 4
	}
	height := 8 + listH
	if height > layoutHeight-2 {
		height = layoutHeight - 2
	}
	if height < 12 {
		height = 12
	}
	return height
}

// drawRunForEachHistoryPickerContent draws the command-history picker body: a query filter row
// followed by the fuzzy-ranked command list. Mirrors drawMassRenamePatternPickerContent
// (mass_rename_pattern_dialog_render.go), reading st.Items[entIdx] directly as the display/search
// line since entries are already plain command strings (no name/description indirection needed).
func drawRunForEachHistoryPickerContent(screen tcell.Screen, rect Rect, st RunForEachHistoryPickerState, borderStyle tcell.Style, styles theme.Theme) {
	innerWidth := draw.DialogContentWidth(rect)
	if innerWidth <= 0 {
		return
	}
	_, dbg, _ := styles.DialogSurface.Decompose()
	textStyle := styles.DialogText.Background(dbg)
	primaryCol := draw.DialogTextX(rect)
	innerBottom := rect.Y + rect.Height - 2

	y := rect.Y + 1
	if y >= innerBottom {
		return
	}
	primitive.Text(screen, primaryCol, y, innerWidth, "Filter:", textStyle)
	y += 2 // blank line between label and input (AGENTS.md dialog input layout)
	if y >= innerBottom {
		return
	}
	queryFocused := st.Focus == 0
	draw.DrawScrollingDialogInput(screen, primaryCol, y, innerWidth, draw.ScrollingInputState{Value: st.Query, Cursor: st.QueryCursor, Scroll: st.QueryScroll}, queryFocused, false, styles)
	y++
	if y >= innerBottom {
		return
	}
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	vp := innerBottom - y - 1 // row innerBottom-1 is the shared separator above the button row
	if vp < 1 {
		vp = 1
	}
	for row := 0; row < vp && y < innerBottom; row++ {
		idxInRank := st.ListScroll + row
		baseStyle := styles.DialogText.Background(dbg)
		line := ""
		var ranges []search.Range
		isCursor := false
		if idxInRank < len(st.Ranked) {
			entIdx := st.Ranked[idxInRank]
			if entIdx >= 0 && entIdx < len(st.Items) {
				line = st.Items[entIdx]
				if entIdx < len(st.MatchRanges) {
					ranges = st.MatchRanges[entIdx]
				}
			}
			isCursor = st.Focus == 0 && idxInRank == st.Selected
		}
		matchStyle := styles.FuzzyHighlight
		if isCursor {
			baseStyle = styles.DialogOptionRowStyle(true, false)
			matchStyle = styles.FuzzyHighlightCursor
		}
		_, bg, _ := baseStyle.Decompose()
		matchStyle = matchStyle.Background(bg)
		text, spans := fuzzyRowContent(line, ranges, innerWidth, matchStyle, false)
		primitive.StyledText(screen, primaryCol, y, innerWidth, text, baseStyle, spans)
		y++
	}
}
