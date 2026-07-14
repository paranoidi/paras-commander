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
) {
	if !fileListActive || chromeBlocked {
		return
	}
	entry, _, ok := state.VisibleEntry(state.Cursor)
	if !ok {
		return
	}
	if !entryDisplayNameTruncated(entry, nameWidth, showIcons, suffix, styles) {
		return
	}
	fullName := entryListingFullName(entry, showIcons)
	fullRunes := []rune(fullName)
	if len(fullRunes) == 0 {
		return
	}
	startX, endX, spanOK := panelBottomCenterOverlaySpan(rect, panelID, ctx)
	if spanOK {
		spanW := endX - startX + 1
		if spanW > 0 && len(fullRunes) <= spanW {
			pad := spanW - len(fullRunes)
			leftPad := pad / 2
			x := startX + leftPad
			y := rect.Y + rect.Height - 1
			primitive.TextOverlay(screen, x, y, len(fullRunes), fullName, titleStyle)
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
) {
	if state.CursorNameHintCoalesce {
		return
	}
	entry, _, ok := state.VisibleEntry(state.Cursor)
	if !ok {
		return
	}
	subtreeMark := entry.Type == localfs.EntryDirectory && nameWidth > 2 && state.HasSelectionInSubtree(entry.Path)
	jobMark, jobStatus := EntryPathJobMarkStatus(entry.Path, jobMarks)
	var jobMarkGlyph rune
	if jobMark {
		if glyphStr := ctx.Styles.SymbolJobsList(jobStatus); glyphStr != "" {
			jobMarkGlyph, _ = utf8.DecodeRuneInString(glyphStr)
		}
	}
	suffix := panellist.RowSuffix{
		JobGlyph:         jobMarkGlyph,
		NewFileTier:      state.NewFileMarkTier(entry),
		SubtreeSelection: subtreeMark,
	}
	drawPanelBottomCursorNameHint(screen, rect, panelID, state, ctx, fileListActive, chromeBlocked, titleStyle, showIcons, nameWidth, suffix, ctx.Styles, fallbackOut)
}
