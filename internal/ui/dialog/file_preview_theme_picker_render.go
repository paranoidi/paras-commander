package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

// DrawFilePreviewThemePicker paints the right-side theme list in F3 file view.
func DrawFilePreviewThemePicker(screen tcell.Screen, rect geom.Rect, state FilePreviewThemePickerState, styles theme.Theme) {
	if !state.Open || len(state.Choices) == 0 || rect.Width < 8 || rect.Height < 8 {
		return
	}

	chrome := styles.PanelChrome(true, false)
	borderStyle := chrome.Frame
	primitive.Box(screen, primitive.Rect(rect), borderStyle)
	inner := primitive.Rect{X: rect.X + 1, Y: rect.Y + 1, Width: rect.Width - 2, Height: rect.Height - 2}
	if inner.Width > 0 && inner.Height > 0 {
		primitive.Fill(screen, inner, ' ', chrome.Surface)
	}

	titleStyle := chrome.Title
	titleX := rect.X + 2
	titleWidth := rect.Width - 4
	if titleWidth < 1 {
		titleWidth = 1
	}
	primitive.TextOverlay(screen, titleX, rect.Y, titleWidth, " Style ", titleStyle)

	_, surfaceBG, _ := chrome.Surface.Decompose()
	leftCol := rect.X + 2
	inputWidth := filePreviewThemePickerQueryWidth(rect)

	draw.DrawScrollingDialogInput(screen, leftCol, rect.Y+1, inputWidth, state.Query, state.QueryCursor, state.QueryScroll, "", true, false, styles)

	sepY := rect.Y + 2
	for x := rect.X + 1; x < rect.X+rect.Width-1; x++ {
		screen.SetContent(x, sepY, '─', nil, borderStyle)
	}

	listH := FilePreviewThemePickerListRows(rect)
	listTop := rect.Y + 3
	rowWidth := inputWidth
	selectedEntIdx := -1
	if state.Selected >= 0 && state.Selected < len(state.Ranked) {
		selectedEntIdx = state.Ranked[state.Selected]
	}
	for row := 0; row < listH; row++ {
		y := listTop + row
		idxInRank := state.ListScroll + row
		if idxInRank >= len(state.Ranked) {
			break
		}
		baseStyle := styles.PanelText.Background(surfaceBG)
		entIdx := state.Ranked[idxInRank]
		line := filePreviewThemePickerRowLabel(state, entIdx)
		var ranges []search.Range
		if entIdx >= 0 && entIdx < len(state.MatchRanges) {
			ranges = state.MatchRanges[entIdx]
		}
		isCursor := idxInRank == state.Selected
		matchStyle := styles.FuzzyHighlight
		if isCursor {
			baseStyle = styles.DialogOptionRowStyle(true, false).Background(surfaceBG)
			matchStyle = styles.FuzzyHighlightCursor.Background(surfaceBG)
		} else {
			_, bg, _ := baseStyle.Decompose()
			matchStyle = matchStyle.Background(bg)
		}
		marker := "( ) "
		if entIdx >= 0 && entIdx == selectedEntIdx {
			marker = "(*) "
		}
		markerWidth := len([]rune(marker))
		labelWidth := rowWidth - markerWidth
		if labelWidth < 1 {
			labelWidth = 1
		}
		primitive.Text(screen, leftCol, y, markerWidth, marker, baseStyle)
		text, spans := fuzzyRowContent(line, ranges, labelWidth, matchStyle, false)
		primitive.StyledText(screen, leftCol+markerWidth, y, labelWidth, text, baseStyle, spans)
	}
}

// EnsureFilePreviewThemePickerListScroll keeps Selected visible in a list of height listRows.
func EnsureFilePreviewThemePickerListScroll(state *FilePreviewThemePickerState, listRows int) {
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

// FilePreviewThemePickerListRows returns how many theme rows fit in the picker panel.
func FilePreviewThemePickerListRows(rect geom.Rect) int {
	if rect.Height < 6 {
		return 1
	}
	rows := rect.Height - 4
	if rows < 1 {
		return 1
	}
	return rows
}

func filePreviewThemePickerQueryWidth(rect geom.Rect) int {
	w := rect.Width - 4
	if w < 1 {
		return 1
	}
	return w
}

func filePreviewThemePickerRowLabel(state FilePreviewThemePickerState, entIdx int) string {
	if entIdx < 0 {
		return ""
	}
	if entIdx < len(state.Choices) {
		if label := state.Choices[entIdx].Label; label != "" {
			return label
		}
		if name := state.Choices[entIdx].Name; name != "" {
			return name
		}
	}
	if entIdx < len(state.DisplayLines) {
		return state.DisplayLines[entIdx]
	}
	return ""
}
