package ui

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// entryDisplayNameTruncated reports whether the listing name column would shorten the entry name.
func entryDisplayNameTruncated(entry localfs.Entry, nameWidth int, showFileIcons bool, suffix panellist.RowSuffix, styles theme.Theme) bool {
	if nameWidth <= 0 {
		return false
	}
	suffixLen := panellist.SuffixDecorationLen(nameWidth, suffix, entry, styles)
	innerW := nameWidth - suffixLen
	if innerW < 1 {
		innerW = 1
	}
	bodyLen := entryDisplayBodyLen(entry, showFileIcons)
	return bodyLen > innerW
}

func entryDisplayBodyLen(entry localfs.Entry, showFileIcons bool) int {
	n := 1 + utf8.RuneCountInString(entry.Name)
	if entry.Type == localfs.EntrySymlink {
		n++
	}
	return n
}

// entryListingFullName is the full name glyph sequence shown in the name column (prefix + name + symlink @).
func entryListingFullName(entry localfs.Entry, showFileIcons bool) string {
	prefix := " "
	if entry.Type == localfs.EntryDirectory && !showFileIcons {
		prefix = "/"
	}
	name := prefix + entry.Name
	if entry.Type == localfs.EntrySymlink {
		name += "@"
	}
	return name
}

// panelBottomCenterOverlaySpan returns the inclusive horizontal span on the bottom interior row
// where a centered cursor-name overlay may be painted without covering indicator labels.
func panelBottomCenterOverlaySpan(rect Rect, panelID int, ctx PanelBottomIndicatorContext) (startX, endX int, ok bool) {
	firstIn := rect.X + 1
	lastIn := rect.X + rect.Width - 2
	if lastIn < firstIn {
		return 0, 0, false
	}
	endReserved := panelBottomEndEdgeReservedStart(rect, ctx)

	if panelID == SecondaryPanel {
		leftBound := firstIn
		if panelBottomEndEdgeTotalWidth(ctx) > 0 && endReserved < lastIn {
			leftBound = endReserved + 1
		}
		leftBound = max(leftBound, panelBottomPhysicalLeftChainEndX(rect, ctx, leftBound, endReserved)+1)
		rightBound := lastIn - panelBottomStartEdgeUsedWidth(rect, panelID, ctx)
		if ctx.SelectionSizeCenterEnd > 0 {
			rightBound = min(rightBound, ctx.SelectionSizeCenterEnd)
		}
		if leftBound > rightBound {
			return 0, 0, false
		}
		return leftBound, rightBound, true
	}

	leftBound := panelBottomStartEdgeEndX(rect, panelID, ctx, firstIn) + 1
	rightBound := endReserved
	leftBound = max(leftBound, panelBottomPhysicalLeftChainEndX(rect, ctx, leftBound, rightBound)+1)

	if leftBound > rightBound {
		return 0, 0, false
	}
	return leftBound, rightBound, true
}

func panelBottomStartEdgeEndX(rect Rect, panelID int, ctx PanelBottomIndicatorContext, firstIn int) int {
	used := panelBottomStartEdgeUsedWidth(rect, panelID, ctx)
	if used == 0 {
		return firstIn - 1
	}
	if panelID == SecondaryPanel {
		return rect.X + rect.Width - 2
	}
	return firstIn + used - 1
}

func panelBottomStartEdgeUsedWidth(rect Rect, panelID int, ctx PanelBottomIndicatorContext) int {
	var startEdge []panelBottomIndicatorSegment
	for _, seg := range collectPanelBottomIndicators(ctx) {
		if seg.Edge == PanelBottomEdgeStart {
			startEdge = append(startEdge, seg)
		}
	}
	available := panelBottomEdgeAvailableWidth(rect, ctx)
	startEdge = dropPanelBottomIndicatorsForWidth(startEdge, available, true)
	if len(startEdge) == 0 {
		return 0
	}
	padW := utf8.RuneCountInString(startEdge[0].Label)
	return 1 + padW
}

func panelBottomPhysicalLeftChainEndX(rect Rect, ctx PanelBottomIndicatorContext, minX, maxX int) int {
	var physicalLeft []panelBottomIndicatorSegment
	for _, seg := range collectPanelBottomIndicators(ctx) {
		if seg.Edge == PanelBottomEdgePhysicalLeft {
			physicalLeft = append(physicalLeft, seg)
		}
	}
	if len(physicalLeft) == 0 {
		return minX - 1
	}
	x := panelBottomPhysicalLeftChainStartX(rect, ctx.SelectionsBottomHint)
	if x > maxX {
		return minX - 1
	}
	maxCols := maxX - x + 1
	if ctx.SelectionSizeCenterStart > 0 {
		maxCols = min(maxCols, ctx.SelectionSizeCenterStart-x)
	}
	leadingDash := !ctx.SelectionsBottomHint
	physicalLeft = dropPanelBottomIndicatorsForWidth(physicalLeft, maxCols, leadingDash)
	if len(physicalLeft) == 0 {
		return minX - 1
	}
	if ctx.SelectionsBottomHint {
		x++
	} else {
		x++
	}
	for i, seg := range physicalLeft {
		if i > 0 {
			x++
		}
		x += utf8.RuneCountInString(seg.Label)
	}
	if x < minX {
		return minX - 1
	}
	return x
}

// CursorNameHintFallback holds the active panel cursor name when it must be shown above
// the footer because it does not fit on the panel bottom border.
type CursorNameHintFallback struct {
	FullName string
	Style    tcell.Style
}

func cursorNameHintFallbackOut(fileListActive bool, out *CursorNameHintFallback) *CursorNameHintFallback {
	if fileListActive {
		return out
	}
	return nil
}

// paintPanelBottomCursorNameOverlay paints fullName centered in [startX, endX] on bottom row y:
// name glyphs first, then border dashes for any remaining span cells (clears a longer prior overlay).
func paintPanelBottomCursorNameOverlay(
	screen tcell.Screen,
	startX, endX, y int,
	fullName string,
	titleStyle, borderStyle tcell.Style,
) bool {
	spanW := endX - startX + 1
	if spanW <= 0 {
		return false
	}
	runes := []rune(fullName)
	if len(runes) == 0 || len(runes) > spanW {
		return false
	}
	leftPad := (spanW - len(runes)) / 2
	for i, r := range runes {
		screen.SetContent(startX+leftPad+i, y, r, nil, titleStyle)
	}
	for col := 0; col < leftPad; col++ {
		screen.SetContent(startX+col, y, '─', nil, borderStyle)
	}
	for col := leftPad + len(runes); col < spanW; col++ {
		screen.SetContent(startX+col, y, '─', nil, borderStyle)
	}
	return true
}

func setCursorNameHintPinned(out *string, value string) {
	if out != nil {
		*out = value
	}
}

func drawPanelBottomCursorNameHint(
	screen tcell.Screen,
	rect Rect,
	panelID int,
	state panel.State,
	ctx PanelBottomIndicatorContext,
	fileListActive bool,
	chromeBlocked bool,
	titleStyle tcell.Style,
	showIcons bool,
	nameWidth int,
	suffix panellist.RowSuffix,
	styles theme.Theme,
	fallbackOut *CursorNameHintFallback,
	pinnedOut *string,
) {
	if !fileListActive || chromeBlocked {
		setCursorNameHintPinned(pinnedOut, "")
		return
	}
	if state.CursorNameHintCoalesce {
		if state.CursorNameHintPinned == "" {
			return
		}
		paintOrFallbackCursorNameHint(screen, rect, panelID, ctx, state.CursorNameHintPinned, titleStyle, fallbackOut)
		return
	}
	entry, _, ok := state.VisibleEntry(state.Cursor)
	if !ok {
		setCursorNameHintPinned(pinnedOut, "")
		return
	}
	if !entryDisplayNameTruncated(entry, nameWidth, showIcons, suffix, styles) {
		setCursorNameHintPinned(pinnedOut, "")
		return
	}
	fullName := entryListingFullName(entry, showIcons)
	if fullName == "" {
		setCursorNameHintPinned(pinnedOut, "")
		return
	}
	paintOrFallbackCursorNameHint(screen, rect, panelID, ctx, fullName, titleStyle, fallbackOut)
	setCursorNameHintPinned(pinnedOut, fullName)
}

func paintOrFallbackCursorNameHint(
	screen tcell.Screen,
	rect Rect,
	panelID int,
	ctx PanelBottomIndicatorContext,
	fullName string,
	titleStyle tcell.Style,
	fallbackOut *CursorNameHintFallback,
) {
	startX, endX, spanOK := panelBottomCenterOverlaySpan(rect, panelID, ctx)
	if spanOK {
		y := rect.Y + rect.Height - 1
		if paintPanelBottomCursorNameOverlay(screen, startX, endX, y, fullName, titleStyle, ctx.BorderStyle) {
			return
		}
	}
	if fallbackOut != nil {
		fallbackOut.FullName = fullName
		fallbackOut.Style = titleStyle
	}
}

// drawCursorNameHintScreenFallback paints a cursor-name hint centered on the row above the footer
// when the full name does not fit on the panel bottom border.
func drawCursorNameHintScreenFallback(screen tcell.Screen, layout Layout, fallback *CursorNameHintFallback, terminalVisible bool) {
	if fallback == nil || fallback.FullName == "" || layout.Footer.Height <= 0 {
		return
	}
	rowY := layout.Footer.Y - 1
	if terminalVisible && layout.Terminal.Height > 0 {
		rowY = layout.Terminal.Y
	}
	drawScreenCenteredCursorNameHint(screen, Rect{X: 0, Y: rowY, Width: layout.Width, Height: 1}, fallback.FullName, fallback.Style)
}

func drawScreenCenteredCursorNameHint(screen tcell.Screen, rect Rect, fullName string, style tcell.Style) {
	if fullName == "" || rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	runes := []rune(fullName)
	if len(runes) > rect.Width {
		fullName = primitive.TruncateRight(fullName, rect.Width)
		runes = []rune(fullName)
	}
	if len(runes) == 0 {
		return
	}
	x := rect.X + (rect.Width-len(runes))/2
	y := rect.Y
	primitive.TextOverlay(screen, x, y, len(runes), fullName, style)
}

func drawPanelCursorNameHintForState(
	screen tcell.Screen,
	rect Rect,
	panelID int,
	state panel.State,
	ctx PanelBottomIndicatorContext,
	fileListActive bool,
	chromeBlocked bool,
	titleStyle tcell.Style,
	showIcons bool,
	nameWidth int,
	jobMarks []JobPathMark,
	fallbackOut *CursorNameHintFallback,
	pinnedOut *string,
) {
	if state.CursorNameHintCoalesce {
		drawPanelBottomCursorNameHint(screen, rect, panelID, state, ctx, fileListActive, chromeBlocked, titleStyle, showIcons, nameWidth, panellist.RowSuffix{}, ctx.Styles, fallbackOut, pinnedOut)
		return
	}
	entry, _, ok := state.VisibleEntry(state.Cursor)
	if !ok {
		setCursorNameHintPinned(pinnedOut, "")
		return
	}
	subtreeMark := entry.Type == localfs.EntryDirectory && nameWidth > 2 && state.HasSelectionInSubtree(entry.Path)
	jobMark, _, jobWrite := EntryPathJobMarkStatus(entry.Path, jobMarks)
	var jobMarkGlyph rune
	if jobMark {
		jobMarkGlyph = ctx.Styles.SymbolFilelistJob()
	}
	suffix := panellist.RowSuffix{
		JobGlyph:         jobMarkGlyph,
		NewFileTier:      state.NewFileMarkTier(entry),
		SubtreeSelection: subtreeMark,
		JobWrite:         jobWrite,
	}
	drawPanelBottomCursorNameHint(screen, rect, panelID, state, ctx, fileListActive, chromeBlocked, titleStyle, showIcons, nameWidth, suffix, ctx.Styles, fallbackOut, pinnedOut)
}
