package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// EnsureMassRenamePatternPickerListScroll keeps Selected row visible in a list of height
// listRows. Shared by the load-pattern and pattern-history pickers.
func EnsureMassRenamePatternPickerListScroll(state *MassRenamePatternPickerState, listRows int) {
	n := len(state.Ranked)
	if n == 0 || listRows <= 0 {
		state.ListScroll = 0
		return
	}
	if state.Selected < 0 {
		state.Selected = 0
	}
	if state.Selected >= n {
		state.Selected = n - 1
	}
	if state.ListScroll > state.Selected {
		state.ListScroll = state.Selected
	}
	if state.Selected >= state.ListScroll+listRows {
		state.ListScroll = state.Selected - listRows + 1
	}
}

// massRenameSavePromptDialogHeight returns the outer dialog height for the save-pattern
// prompt: two fields (Name, Description), each laid out as label / blank / input / blank
// (mirrors the generic multi-field dialog height formula, len(Fields)*4+4).
func massRenameSavePromptDialogHeight() int {
	return 2*4 + 4
}

// drawMassRenameSavePromptContent draws the Name/Description save-pattern prompt body. It
// reuses drawMultiFieldDialog verbatim: at this phase d.Fields holds exactly the two
// {Name, Description} fields set up by openMassRenameSavePrompt, laid out the same way any
// other two-field file dialog is (label row -> blank row -> input row).
func drawMassRenameSavePromptContent(screen tcell.Screen, rect Rect, state FileDialogState, borderStyle tcell.Style, styles theme.Theme) {
	drawMultiFieldDialog(screen, rect, state, styles)
}

// massRenamePatternPickerDialogHeight returns the outer dialog height for the load-pattern and
// pattern-history pickers (identical layout): query label + blank + query input + separator +
// fuzzy list + (shared separator and button row are added by DrawFileDialog /
// drawOkCancelButtons). Mirrors DrawHistoryDialog's list-height clamp.
func massRenamePatternPickerDialogHeight(layoutHeight int) int {
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

// drawMassRenamePatternPickerContent draws the load-pattern/pattern-history picker body: a query
// filter row followed by the fuzzy-ranked list. Mirrors DrawHistoryDialog's layout, sized
// dynamically from rect (like drawMassRenameDialog's preview list) rather than a separately
// duplicated row count, so it always matches whatever height FileDialogRect computed. Shared by
// both MassRenamePhaseLoadPicker and MassRenamePhaseHistoryPicker; the caller passes whichever
// of state.MassRenameLoadPicker / state.MassRenameHistoryPicker matches the current phase.
func drawMassRenamePatternPickerContent(screen tcell.Screen, rect Rect, st MassRenamePatternPickerState, borderStyle tcell.Style, styles theme.Theme) {
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
				line = MassRenamePatternSearchLine(st.Items[entIdx])
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
