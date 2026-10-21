package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// EnsureHistoryListScroll keeps Selected row visible in a list of height listRows.
func EnsureHistoryListScroll(state *HistoryDialogState, listRows int) {
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

// HistoryZoxideMnemonic toggles the "Include zoxide" checkbox (Alt+I).
const HistoryZoxideMnemonic = 'I'

// historySourcePrefixWidth is the "<icon> " source prefix on each History row.
const historySourcePrefixWidth = 2

// HistoryDialogListRows returns the History dialog's list height for a layout height, or 0
// when the dialog does not fit. zoxideRow adds the "Include zoxide" checkbox row.
func HistoryDialogListRows(layoutHeight int, zoxideRow bool) int {
	chrome := 6
	if zoxideRow {
		chrome++
	}
	listH := min(max(layoutHeight-12, 4), 18)
	if chrome+listH > layoutHeight-2 {
		listH = layoutHeight - 2 - chrome
		if listH < 4 {
			return 0
		}
	}
	return listH
}

func DrawHistoryDialog(screen tcell.Screen, layout Layout, state HistoryDialogState, styles theme.Theme, rowMarks RowMarksFunc) {
	width := 78
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < 36 {
		return
	}

	listH := HistoryDialogListRows(layout.Height, state.ZoxideAvailable)
	if listH == 0 {
		return
	}
	extra := 0
	if state.ZoxideAvailable {
		extra = 1
	}
	height := 6 + extra + listH

	rect := draw.CenteredDialogRect(layout, width, height)
	title := "History"
	if state.BothPanels {
		title = "History - Both panels"
	}
	borderStyle := draw.DrawDialogFrame(screen, rect, title, styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	itemBg := dbg
	primaryCol := rect.X + 2
	inputWidth := rect.Width - 4

	filterFocused := state.Focus == 0
	draw.DrawScrollingDialogInput(screen, primaryCol, rect.Y+1, inputWidth, draw.ScrollingInputState{Value: state.Query, Cursor: state.QueryCursor, Scroll: state.QueryScroll, LeadingIcon: styles.IconSearchIcon()}, filterFocused, false, styles)

	if state.ZoxideAvailable {
		draw.DrawDialogCheckbox(screen, draw.DialogOptionX(rect), rect.Y+2, "Include zoxide", HistoryZoxideMnemonic, state.IncludeZoxide, state.Focus == state.HistoryZoxideFocus(), false, styles)
	}

	sepBeforeList := rect.Y + 2 + extra
	draw.DrawDialogHSeparator(screen, rect, sepBeforeList, borderStyle)

	listTop := rect.Y + 3 + extra
	rowWidth := inputWidth
	for row := 0; row < listH; row++ {
		y := listTop + row
		idxInRank := state.ListScroll + row
		baseStyle := styles.DialogText.Background(itemBg)
		line := ""
		var ranges []search.Range
		isCursor := false
		missing := false
		absPath := ""
		if idxInRank < len(state.Ranked) {
			entIdx := state.Ranked[idxInRank]
			if entIdx >= 0 && entIdx < len(state.DisplayLines) {
				line = state.DisplayLines[entIdx]
				if entIdx < len(state.MatchRanges) {
					ranges = state.MatchRanges[entIdx]
				}
			}
			if entIdx >= 0 && entIdx < len(state.PathMissing) {
				missing = state.PathMissing[entIdx]
			}
			if entIdx >= 0 && entIdx < len(state.Paths) {
				absPath = state.Paths[entIdx]
			}
			isCursor = state.Focus == 0 && idxInRank == state.Selected
		}
		matchStyle := styles.FuzzyHighlight
		switch {
		case isCursor:
			baseStyle = styles.DialogOptionRowStyle(true, false)
			matchStyle = styles.FuzzyHighlightCursor
		case missing:
			baseStyle = styles.DialogOptionInvalidStyle()
		}
		_, bg, _ := baseStyle.Decompose()
		matchStyle = matchStyle.Background(bg)
		marks := RowMarks{}
		if rowMarks != nil {
			marks = rowMarks(absPath)
		}
		marks.Missing, marks.Cursor = missing, isCursor
		pathCol, pathWidth := primaryCol, rowWidth
		if absPath != "" {
			// Source prefix: zoxide rows vs the app's own panel history.
			icon := styles.IconDialogHistory()
			if state.Ranked[idxInRank] >= state.ZoxideFrom {
				icon = styles.IconDialogZoxide()
			}
			fg, _, _ := styles.DialogOptionRowStyle(false, false).Decompose()
			if isCursor {
				fg = styles.DialogMissingIconForeground(true)
			}
			primitive.Text(screen, primaryCol, y, historySourcePrefixWidth, icon+" ", baseStyle.Foreground(fg))
			pathCol += historySourcePrefixWidth
			pathWidth -= historySourcePrefixWidth
		}
		marksW := RowMarksWidth(marks)
		fitWidth := pathWidth - marksW
		if fitWidth < 1 {
			fitWidth = 1
		}
		text, spans := fuzzyRowContent(line, ranges, fitWidth, matchStyle, false)
		primitive.StyledText(screen, pathCol, y, pathWidth, text, baseStyle, spans)
		if marksW > 0 {
			// Marks follow the path, not the row end (same as find dialog).
			DrawRowMarksSuffix(screen, pathCol+min(runewidth.StringWidth(text), fitWidth), y, marksW, marks, bg, styles)
		}
	}

	sepAfterList := listTop + listH
	draw.DrawDialogHSeparator(screen, rect, sepAfterList, borderStyle)

	buttonY := rect.Y + rect.Height - 2
	okFocused := state.Focus == state.HistoryOKFocus()
	cancelFocused := state.Focus == state.HistoryCancelFocus()
	draw.DrawOKCancelButtonRow(screen, rect, buttonY, okFocused, cancelFocused, styles)
}
