package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// EnsurePathPickerListScroll keeps Selected row visible in a list of height listRows.
func EnsurePathPickerListScroll(state *PathPickerState, listRows int) {
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

func DrawPathPickerDialog(screen tcell.Screen, layout Layout, state PathPickerState, styles theme.Theme) {
	width := 78
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < 36 {
		return
	}

	listH := layout.Height - 12
	switch {
	case listH > 18:
		listH = 18
	case listH < 4:
		listH = 4
	}
	height := 8 + listH
	if height > layout.Height-2 {
		height = layout.Height - 2
		listH = height - 8
		if listH < 4 {
			return
		}
	}

	title := state.Title
	if title == "" {
		title = "Choose path"
	}

	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, title, styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	itemBg := dbg
	leftCol := rect.X + 2
	inputWidth := rect.Width - 4

	primitive.Text(screen, leftCol, rect.Y+1, inputWidth, "Filter:", styles.DialogText.Background(itemBg))

	filterFocused := state.Focus == 0
	inputInvalid := state.QueryPathInvalid && !state.QueryPathCheckPending
	draw.DrawScrollingDialogInput(screen, leftCol, rect.Y+3, inputWidth, state.Query, state.QueryCursor, state.QueryScroll, state.QueryCompletionSuffix, filterFocused, inputInvalid, styles)

	sepBeforeList := rect.Y + 4
	draw.DrawDialogHSeparator(screen, rect, sepBeforeList, borderStyle)

	listTop := rect.Y + 5
	rowWidth := inputWidth
	sourceW, nameW, pathW := pathPickerColumnWidths(state.Items, rowWidth)
	for row := 0; row < listH; row++ {
		y := listTop + row
		idxInRank := state.ListScroll + row
		baseStyle := styles.DialogText.Background(itemBg)
		var item PathPickerItem
		var ranges []search.Range
		isCursor := false
		if idxInRank < len(state.Ranked) {
			entIdx := state.Ranked[idxInRank]
			if entIdx >= 0 && entIdx < len(state.Items) {
				item = state.Items[entIdx]
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
		_, bg, _ := baseStyle.Decompose()
		matchStyle = matchStyle.Background(bg)
		text, spans := pathPickerRowContent(item, ranges, sourceW, nameW, pathW, matchStyle)
		primitive.StyledText(screen, leftCol, y, rowWidth, text, baseStyle, spans)
	}

	sepAfterList := listTop + listH
	draw.DrawDialogHSeparator(screen, rect, sepAfterList, borderStyle)

	buttonY := rect.Y + rect.Height - 2
	okFocused := state.Focus == 1
	cancelFocused := state.Focus == 2
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: okFocused},
		{Label: "Cancel", Shortcut: 'C', Focused: cancelFocused},
	}, styles)
}
