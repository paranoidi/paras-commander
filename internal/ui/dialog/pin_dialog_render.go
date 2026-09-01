package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// PinDialogFixedChromeRows is the outer height consumed by rows that aren't the list:
// top border, query row, separator, one mandatory blank row above the bottom border,
// bottom border. No OK/Cancel row — Esc closes, S-Left/S-Right/F8/Enter cover every
// action a button row would (mirrors History's chrome but without its OK/Cancel row).
// Exported so internal/app can size the dialog identically instead of duplicating the
// row count as a separate magic number (see historyDialogListRows/DrawHistoryDialog's
// 7-vs-6 drift this project already has for History; don't reproduce it here).
const PinDialogFixedChromeRows = 5

// PinDialogListRows returns the list row count for a dialog sized to layout, mirroring the
// History/PathPicker dialogs' clamp shape (generous outer margin, 4..18 rows). Exported so
// internal/app can size the dialog identically instead of duplicating this computation.
func PinDialogListRows(layoutHeight int) int {
	listH := layoutHeight - 12
	switch {
	case listH > 18:
		listH = 18
	case listH < 4:
		listH = 4
	}
	if PinDialogFixedChromeRows+listH > layoutHeight-2 {
		listH = layoutHeight - 2 - PinDialogFixedChromeRows
		if listH < 4 {
			return 4
		}
	}
	return listH
}

// EnsurePinListScroll keeps Selected row visible in a list of height listRows.
func EnsurePinListScroll(state *PinDialogState, listRows int) {
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

// DrawPinDialog paints the Pin dialog: a fuzzy-filterable, most-recently-pinned-first
// list of pinned files/directories. No OK/Cancel buttons (Esc closes, mirroring History).
func DrawPinDialog(screen tcell.Screen, layout Layout, state PinDialogState, items []PinDialogItem, styles theme.Theme) {
	width := 78
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < 36 {
		return
	}

	listH := PinDialogListRows(layout.Height)
	height := PinDialogFixedChromeRows + listH
	if height > layout.Height-2 {
		height = layout.Height - 2
		listH = height - PinDialogFixedChromeRows
		if listH < 4 {
			return
		}
	}

	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, "Pin", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	primaryCol := draw.DialogTextX(rect)
	rowWidth := draw.DialogContentWidth(rect)

	draw.DrawScrollingDialogInput(screen, primaryCol, rect.Y+1, rowWidth, draw.ScrollingInputState{Value: state.Query, Cursor: state.QueryCursor, Scroll: state.QueryScroll, LeadingSymbol: styles.SymbolSearchIcon()}, true, false, styles)

	sepBeforeList := rect.Y + 2
	draw.DrawDialogHSeparator(screen, rect, sepBeforeList, borderStyle)

	listTop := rect.Y + 3
	for row := 0; row < listH; row++ {
		y := listTop + row
		idxInRank := state.ListScroll + row
		baseStyle := styles.DialogText.Background(dbg)
		var item PinDialogItem
		var ranges []search.Range
		haveItem := false
		if idxInRank < len(state.Ranked) {
			entIdx := state.Ranked[idxInRank]
			if entIdx >= 0 && entIdx < len(items) {
				item = items[entIdx]
				haveItem = true
				if entIdx < len(state.MatchRanges) {
					ranges = state.MatchRanges[entIdx]
				}
			}
		}
		isCursor := haveItem && idxInRank == state.Selected
		switch {
		case isCursor:
			baseStyle = styles.DialogOptionRowStyle(true, false)
		case haveItem && item.PathMissing:
			baseStyle = styles.DialogOptionInvalidStyle()
		}
		if !haveItem {
			primitive.Text(screen, primaryCol, y, rowWidth, "", baseStyle)
			continue
		}
		matchStyle := styles.FuzzyHighlight
		if isCursor {
			matchStyle = styles.FuzzyHighlightCursor
		}
		_, bg, _ := baseStyle.Decompose()
		matchStyle = matchStyle.Background(bg)
		drawPinRow(screen, primaryCol, y, rowWidth, item, ranges, baseStyle, matchStyle, styles)
	}
}

// drawPinRow renders one pin row as "<glyph> <path>", the path fuzzy-highlighted and
// fit (middle-ellipsized) to whatever width remains after the glyph and its trailing
// space. The glyph itself carries no highlight — matches are keyed against Path text.
func drawPinRow(screen tcell.Screen, x, y, rowWidth int, item PinDialogItem, ranges []search.Range, baseStyle, matchStyle tcell.Style, styles theme.Theme) {
	glyph := styles.SymbolFile()
	if item.IsDir {
		glyph = styles.SymbolFolder()
	}
	if glyph == "" {
		text, spans := fuzzyRowContent(item.Path, ranges, rowWidth, matchStyle, true)
		primitive.StyledText(screen, x, y, rowWidth, text, baseStyle, spans)
		return
	}
	glyphW := runewidth.RuneWidth([]rune(glyph)[0])
	prefixW := glyphW + 1
	if prefixW > rowWidth {
		prefixW = rowWidth
	}
	primitive.Text(screen, x, y, prefixW, glyph+" ", baseStyle)
	pathWidth := rowWidth - prefixW
	if pathWidth <= 0 {
		return
	}
	text, spans := fuzzyRowContent(item.Path, ranges, pathWidth, matchStyle, true)
	primitive.StyledText(screen, x+prefixW, y, pathWidth, text, baseStyle, spans)
}
