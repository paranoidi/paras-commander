package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// EnsureSFTPConnectListScroll keeps Selected visible in the host list.
func EnsureSFTPConnectListScroll(state *SFTPConnectDialogState, listRows int) {
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

func DrawSFTPConnectDialog(screen tcell.Screen, layout Layout, state SFTPConnectDialogState, styles theme.Theme) {
	width := 78
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < 40 {
		return
	}

	listH := layout.Height - 16
	switch {
	case listH > 12:
		listH = 12
	case listH < 3:
		listH = 3
	}
	height := 12 + listH
	if height > layout.Height-2 {
		height = layout.Height - 2
		listH = height - 12
		if listH < 3 {
			listH = 3
		}
	}

	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, "SFTP", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	itemBg := dbg
	primaryCol := rect.X + 2
	inputWidth := rect.Width - 4

	primitive.Text(screen, primaryCol, rect.Y+1, inputWidth, "SSH config hosts:", styles.DialogText.Background(itemBg))

	filterFocused := state.Focus == 0
	draw.DrawScrollingDialogInput(screen, primaryCol, rect.Y+3, inputWidth, draw.ScrollingInputState{Value: state.Query, Cursor: state.QueryCursor, Scroll: state.QueryScroll}, filterFocused, false, styles)

	sepBeforeList := rect.Y + 4
	draw.DrawDialogHSeparator(screen, rect, sepBeforeList, borderStyle)

	listTop := rect.Y + 5
	rowWidth := inputWidth
	for row := 0; row < listH; row++ {
		y := listTop + row
		idxInRank := state.ListScroll + row
		baseStyle := styles.DialogText.Background(itemBg)
		line := ""
		var ranges []search.Range
		isCursor := false
		if idxInRank < len(state.Ranked) {
			entIdx := state.Ranked[idxInRank]
			if entIdx >= 0 && entIdx < len(state.DisplayLines) {
				line = state.DisplayLines[entIdx]
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
		_, bg, _ := baseStyle.Decompose()
		matchStyle = matchStyle.Background(bg)
		text, spans := fuzzyRowContent(line, ranges, rowWidth, matchStyle, false)
		primitive.StyledText(screen, primaryCol, y, rowWidth, text, baseStyle, spans)
	}

	sep1 := listTop + listH
	draw.DrawDialogHSeparator(screen, rect, sep1, borderStyle)

	locLabelY := sep1 + 1
	primitive.Text(screen, primaryCol, locLabelY, inputWidth, "Location:", styles.DialogText.Background(itemBg))
	inputY := locLabelY + 2
	drawInputField(screen, primaryCol, inputY, inputWidth, state.Location, state.Focus == 1, styles)
	draw.DrawDialogHSeparator(screen, rect, inputY+1, borderStyle)

	btnY := rect.Y + rect.Height - 2
	draw.DrawOKCancelButtonRow(screen, rect, btnY, state.Focus == 2, state.Focus == 3, styles)
}
