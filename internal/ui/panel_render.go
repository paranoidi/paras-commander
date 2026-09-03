package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Layout after the name column: one space, size (panelListSizeCells), optional two spaces + third column.
// When meta is active the column order is: name | meta | size | (mtime | perm | none).
const (
	panelListModTimeCells = 16
	panelListPermCells    = 10
	panelListSizeCells    = 5
	// panelListMetaMax is the maximum display width (terminal cells) for the rendered Meta column
	// (single-cell legacy or tab/newline multi-field layout; see layoutMetaCells).
	panelListMetaMax = 20
	// panelListMetaMinCells is the minimum width of the Meta column ("Meta" header).
	panelListMetaMinCells = 4
)

func panelListReservedAfterName(f panel.ListFormat, nameOnly bool) int {
	if nameOnly {
		return 0
	}
	switch panel.EffectiveListFormat(f) {
	case panel.ListFormatBrief:
		return 1 + panelListSizeCells
	case panel.ListFormatPerm:
		return 1 + panelListSizeCells + 2 + panelListPermCells
	default:
		return 1 + panelListSizeCells + 2 + panelListModTimeCells
	}
}

func panelListThirdColumnWidth(f panel.ListFormat, nameOnly bool) int {
	if nameOnly {
		return 0
	}
	switch panel.EffectiveListFormat(f) {
	case panel.ListFormatBrief:
		return 0
	case panel.ListFormatPerm:
		return panelListPermCells
	default:
		return panelListModTimeCells
	}
}

func panelListNameWidth(rowTextWidth int, f panel.ListFormat, nameOnly, showGit bool) int {
	return max(1, rowTextWidth-panelListReservedBeforeName(showGit, nameOnly)-panelListReservedAfterName(f, nameOnly))
}

// panelListNameWidthWithMeta returns name width when the Meta column is shown.
func panelListNameWidthWithMeta(rowTextWidth, metaColW int, f panel.ListFormat, nameOnly, showGit bool) int {
	return max(1, rowTextWidth-panelListReservedBeforeName(showGit, nameOnly)-panelListReservedAfterName(f, nameOnly)-2-metaColW)
}

func truncateHeaderRunes(max int, s string) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func panelListHeaderTitleWithSortArrow(nameTitle, sizeTitle, thirdTitle string) string {
	for _, s := range []string{nameTitle, sizeTitle, thirdTitle} {
		if strings.ContainsRune(s, '↑') || strings.ContainsRune(s, '↓') {
			return strings.TrimSpace(s)
		}
	}
	return strings.TrimSpace(nameTitle)
}

// PanelStyleConfig carries visual-presentation inputs to drawPanel.
type PanelStyleConfig struct {
	Styles             theme.Theme
	ScrollbarStyle     uiscrollbar.Style
	PreviewChromaStyle string
}

// PanelContext carries panel-identity, focus, and twin-panel coordination inputs to drawPanel.
type PanelContext struct {
	PanelID        int
	FileListActive bool
	// CursorRowActive selects the cursor row's active/inactive highlight and icon key
	// independent of FileListActive (which also drives border/header chrome). Floating
	// overlays like the bottom function menu dim only the row indicator on the active
	// panel while leaving chrome untouched, so this is normally equal to FileListActive
	// but can be forced false without affecting chrome.
	CursorRowActive           bool
	ChromeBlocked             bool
	ActivePanel               int
	OtherPanelPath            string
	HideInactivePanel         bool
	SyncDriverPanelID         int
	QuickViewDriverPanelID    int
	SplitOrientation          SplitOrientation
	SelectionsBottomHint      bool
	ShowSelectionSizeOnBottom bool
	// IsTransferTarget marks this panel as the resolved Copy/Move/Flatten destination
	// (its border is painted with theme.PanelTargetFrame instead of the normal frame).
	IsTransferTarget bool
	// ViMotionActive marks this panel as the active panel while vi-motion mode is on
	// (its border is painted with theme.PanelViMotionFrame instead of the normal frame,
	// unless IsTransferTarget also applies — transfer-target takes priority).
	ViMotionActive bool
	// CursorNameHintFallbackOut receives the full cursor name when it does not fit on the
	// panel bottom border and should be painted above the footer instead.
	CursorNameHintFallbackOut *CursorNameHintFallback
	// CursorNameHintPinnedOut, when non-nil, receives latch updates for the bottom-border
	// overlay text (points at the live panel.State.CursorNameHintPinned in App.model, not a
	// paint-time Model snapshot copy).
	CursorNameHintPinnedOut *string
	// TitlePath, when non-empty, is the left title path instead of state.PathString().
	TitlePath string
	// TitleEndLabel, when non-empty, replaces volume free-space on the title end (title style).
	TitleEndLabel string
}

// PanelDisplayConfig carries feature-flag and data inputs to drawPanel.
type PanelDisplayConfig struct {
	ShowIcons                       bool
	UserHomeDir                     string
	Painter                         DiskUsagePainter
	DiskUsageDescendIntoMountPoints bool
	DiskUsageGoduIgnore             func(string) bool
	ShowDiskUsage                   bool
	JobMarks                        []JobPathMark
	PreviewPrefetchLoading          map[string]struct{}
	PreviewPrefetchWarm             map[string]struct{}
	// PinnedPaths lists absolute paths currently in the app's pin list, for the row-suffix pin
	// glyph; see ui.PinnedPathSet.
	PinnedPaths           map[string]struct{}
	MetaColumns           []MetaColumnState
	ShrunkenShowsNameOnly bool
	ScrollbarShowInactive bool
	CarouselLayout        panelcarousel.Layout
	CarouselFilePreview   FilePreviewState
}

func drawPanel(screen tcell.Screen, rect Rect, state panel.State, panelStyle PanelStyleConfig, ctx PanelContext, display PanelDisplayConfig) {
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	chrome := panelStyle.Styles.PanelChrome(ctx.FileListActive, ctx.ChromeBlocked)
	borderStyle := chrome.Frame
	if ctx.IsTransferTarget {
		borderStyle = panelStyle.Styles.PanelTargetFrame
	} else if ctx.ViMotionActive {
		borderStyle = panelStyle.Styles.PanelViMotionFrame
	}
	titleStyle := chrome.Title
	headerStyle := chrome.Header
	headerCarouselStyle := chrome.HeaderCarousel

	primitive.Box(screen, primitive.Rect(rect), borderStyle, primitive.SharpBorder)
	var selectionSizeLabel string
	if ctx.ShowSelectionSizeOnBottom && state.SelectedPathCount() > 0 {
		selectionSizeLabel, _ = SelectionSizeLabel(
			&state,
			state.Path.IsRemote(),
			display.Painter,
			display.DiskUsageDescendIntoMountPoints,
			display.DiskUsageGoduIgnore,
			panelStyle.Styles.SymbolWorking(),
		)
	}
	jobWriteMark, jobWriteStatus := PanelInsideJobWriteTree(state.PathString(), display.JobMarks)
	bottomCtx := PanelBottomIndicatorContext{
		PanelID:                ctx.PanelID,
		State:                  state,
		SelectionsBottomHint:   ctx.SelectionsBottomHint,
		SyncDriverPanelID:      ctx.SyncDriverPanelID,
		QuickViewDriverPanelID: ctx.QuickViewDriverPanelID,
		HideInactivePanel:      ctx.HideInactivePanel,
		ActivePanel:            ctx.ActivePanel,
		OtherPanelPath:         ctx.OtherPanelPath,
		UserHomeDir:            display.UserHomeDir,
		FileListActive:         ctx.FileListActive,
		ChromeBlocked:          ctx.ChromeBlocked,
		BorderStyle:            borderStyle,
		Styles:                 panelStyle.Styles,
		SelectionSizeLabel:     selectionSizeLabel,
		SplitOrientation:       ctx.SplitOrientation,
		JobWriteMark:           jobWriteMark,
		JobWriteStatus:         jobWriteStatus,
	}
	finalizeBottomCtx(rect, &bottomCtx)
	drawPanelBottomIndicators(screen, rect, bottomCtx)
	inner := primitive.Rect{X: rect.X + 1, Y: rect.Y + 1, Width: rect.Width - 2, Height: rect.Height - 2}
	if inner.Width > 0 && inner.Height > 0 {
		primitive.Fill(screen, inner, ' ', chrome.Surface)
	}
	titleX := rect.X + 2
	innerRight := rect.X + rect.Width - 2
	contentCols := innerRight - titleX + 1
	if contentCols < 1 {
		contentCols = 1
	}
	if ctx.FileListActive && (state.Filter.Active || state.Filter.Editing) {
		inputStyle := panelStyle.Styles.FuzzyInput
		if state.Filter.Active && !state.FilterHasMatches() {
			inputStyle = panelStyle.Styles.FuzzyInputNomatch
		}
		display := "> " + state.Filter.Query
		if state.Filter.Editing {
			cursorCol := 2 + state.Filter.Cursor
			if cursorCol >= len([]rune(display)) {
				// Widen past end-of-text so the caret span lands on a real cell
				// (StyledText's trailing pad columns don't apply spans).
				display += " "
			}
			primitive.StyledText(screen, titleX, rect.Y, contentCols, display, inputStyle, []primitive.Span{
				{Start: cursorCol, End: cursorCol + 1, Style: inputStyle.Reverse(true)},
			})
		} else {
			primitive.Text(screen, titleX, rect.Y, contentCols, display, inputStyle)
		}
	} else {
		titlePath := state.PathString()
		if ctx.TitlePath != "" {
			titlePath = ctx.TitlePath
		}
		endLabel := panelVolumeFreeSpaceTitle(state.VolumeSpaceOK, state.VolumeAvailBytes, state.VolumeTotalBytes)
		endStyle := chrome.DiskUsageOverview
		volumeDecorated := true
		if ctx.TitleEndLabel != "" {
			endLabel = ctx.TitleEndLabel
			endStyle = titleStyle
			volumeDecorated = false
		}
		paintPanelTopTitleRow(screen, titleX, innerRight, contentCols, rect.Y,
			titlePath, display.UserHomeDir,
			panelTitleStyles{Path: titleStyle, End: endStyle, Border: borderStyle},
			endLabel, volumeDecorated)
	}

	visibleRows := PanelListRows(rect)
	if visibleRows == 0 {
		return
	}

	if state.CarouselMode {
		if drawPanelCarousel(screen, panelCarouselParams{
			Rect: rect, State: state, PanelStyle: panelStyle, Ctx: ctx, Display: display,
			BottomCtx: bottomCtx, BorderStyle: borderStyle, TitleStyle: titleStyle,
			HeaderStyle: headerStyle, HeaderCarouselStyle: headerCarouselStyle, SurfaceStyle: chrome.Surface,
			SelectionSizeLabel: selectionSizeLabel, VisibleRows: visibleRows,
		}) {
			return
		}
	}

	layoutRes := panelColumnLayout(rect, state, display)
	leftGutter, iconStrip, nameOnlyDisplay := layoutRes.LeftGutter, layoutRes.IconStrip, layoutRes.NameOnlyDisplay
	showGit, gitStrip, rowTextWidth := layoutRes.ShowGit, layoutRes.GitStrip, layoutRes.RowTextWidth
	gitStart, iconStart, diskDenom := layoutRes.GitStart, layoutRes.IconStart, layoutRes.DiskDenom

	metaLayouts, metaTotalW := LayoutMetaColumns(display.MetaColumns)
	showMeta := len(metaLayouts) > 0
	showMetaEffective := showMeta && !nameOnlyDisplay
	listTextWidth := rowTextWidth
	// In tree mode, panelColumnLayout zeroes iconStrip (icons move into each row's tree gutter
	// instead of a shared strip), but drawPanelRow still reserves panelIconStripCells of gutter
	// width at depth 0 (see treeGutterWidth there). The header has no per-depth gutter of its own,
	// so shift/narrow it by that same fixed amount — this is the single place that offset is
	// computed; row drawing keeps deriving its own (per-depth) gutter independently.
	headerTreeGutter := 0
	if state.ListLayout == panel.ListLayoutTree && panelIconStripCells > 0 && panelIconStripCells < listTextWidth {
		headerTreeGutter = panelIconStripCells
	}
	headerTextWidth := listTextWidth - headerTreeGutter
	header := panelListHeader(headerTextWidth, state, display.ShowIcons, showMetaEffective, metaLayouts, nameOnlyDisplay, showGit)
	headerY := rect.Y + 1
	if leftGutter > 0 {
		for i := 0; i < leftGutter; i++ {
			screen.SetContent(rect.X+1+i, headerY, ' ', nil, headerStyle)
		}
	}
	if showGit {
		paintGitHeader(screen, gitStart, headerY, headerStyle, panelStyle.Styles)
	}
	if display.ShowIcons && state.ListLayout != panel.ListLayoutTree {
		paintPanelIconStripBlank(screen, iconStart, headerY, headerStyle)
	}
	listContentStart := iconStart + iconStrip
	if headerTreeGutter > 0 {
		for i := 0; i < headerTreeGutter; i++ {
			screen.SetContent(listContentStart+i, headerY, ' ', nil, headerStyle)
		}
	}
	primitive.Text(screen, listContentStart+headerTreeGutter, headerY, headerTextWidth, header, headerStyle)

	listFmt := panel.EffectiveListFormat(state.ListFormat)
	// listTextWidth already has the git strip excluded; pass false to avoid double-subtracting.
	nameWidth := panelListNameWidth(listTextWidth, listFmt, nameOnlyDisplay, false)
	if showMetaEffective {
		nameWidth = panelListNameWidthWithMeta(listTextWidth, metaTotalW, listFmt, nameOnlyDisplay, false)
	}
	rowOpts := panelRowOpts{
		ShowIcons: display.ShowIcons,
		ShowMeta:  showMetaEffective,
		MetaColW:  metaTotalW,
		ListFmt:   listFmt,
		NameOnly:  nameOnlyDisplay,
		ShowGit:   showGit,
	}
	for row := 0; row < visibleRows; row++ {
		drawPanelRow(screen, row, panelRowParams{
			Rect: rect, State: state, PanelStyle: panelStyle, Ctx: ctx, Display: display,
			RowOpts: rowOpts, ListTextWidth: listTextWidth, ListContentStart: listContentStart, NameWidth: nameWidth,
			LeftGutter: leftGutter, GitStrip: gitStrip, IconStrip: iconStrip,
			GitStart: gitStart, IconStart: iconStart, ShowGit: showGit,
			DiskDenom:   diskDenom,
			MetaLayouts: metaLayouts, ShowMetaEffective: showMetaEffective,
		})
	}

	drawPanelListScrollbar(screen, rect,
		panelScrollPos{ListTopY: rect.Y + 2, Visible: visibleRows, Total: state.VisibleEntryCount(), Offset: state.ScrollOffset},
		panelStyle.ScrollbarStyle, panelScrollbarShow(ctx.FileListActive, display.ScrollbarShowInactive),
		ctx.FileListActive, ctx.ChromeBlocked, borderStyle, panelStyle.Styles)

	if selectionSizeLabel != "" {
		drawPanelBottomSelectionSize(screen, rect, ctx.PanelID, bottomCtx)
	} else {
		drawPanelCursorNameHintForState(screen, rect, ctx.PanelID, state, bottomCtx, ctx.FileListActive, ctx.ChromeBlocked, titleStyle, display.ShowIcons, nameWidth, display.JobMarks, ctx.CursorNameHintFallbackOut, ctx.CursorNameHintPinnedOut)
	}
}

// panelCarouselParams carries drawPanel's locals needed to paint the carousel-mode
// (parent/center/child column) listing. Field names mirror the drawPanel locals they came
// from so the body below (moved verbatim) can alias them back in one destructuring line.
type panelCarouselParams struct {
	Rect                Rect
	State               panel.State
	PanelStyle          PanelStyleConfig
	Ctx                 PanelContext
	Display             PanelDisplayConfig
	BottomCtx           PanelBottomIndicatorContext
	BorderStyle         tcell.Style
	TitleStyle          tcell.Style
	HeaderStyle         tcell.Style
	HeaderCarouselStyle tcell.Style
	SurfaceStyle        tcell.Style
	SelectionSizeLabel  string
	VisibleRows         int
}

// panelColumnLayoutResult holds the leading-column width/gutter/strip arithmetic shared by
// the header row and the per-row paint loop in drawPanel.
type panelColumnLayoutResult struct {
	LeftGutter      int
	IconStrip       int
	NameOnlyDisplay bool
	ShowGit         bool
	GitStrip        int
	RowTextWidth    int
	GitStart        int
	IconStart       int
	DiskDenom       int64
}

// panelRowParams carries drawPanel's locals needed to paint one row of the classic
// (non-carousel) single-column listing.
type panelRowParams struct {
	Rect              Rect
	State             panel.State
	PanelStyle        PanelStyleConfig
	Ctx               PanelContext
	Display           PanelDisplayConfig
	RowOpts           panelRowOpts
	ListTextWidth     int
	ListContentStart  int
	NameWidth         int
	LeftGutter        int
	GitStrip          int
	IconStrip         int
	GitStart          int
	IconStart         int
	ShowGit           bool
	DiskDenom         int64
	MetaLayouts       []MetaColumnLayout
	ShowMetaEffective bool
}

// drawPanelRow paints one row (row is the 0-based offset from the first visible row) of the
// classic single-column panel listing, moved verbatim out of drawPanel's per-row loop body.
func drawPanelRow(screen tcell.Screen, row int, p panelRowParams) {
	rect, state, panelStyle, ctx, display := p.Rect, p.State, p.PanelStyle, p.Ctx, p.Display
	listTextWidth, listContentStart, nameWidth := p.ListTextWidth, p.ListContentStart, p.NameWidth
	leftGutter, gitStrip, iconStrip := p.LeftGutter, p.GitStrip, p.IconStrip
	showGit := p.ShowGit
	diskDenom := p.DiskDenom
	metaLayouts, showMetaEffective := p.MetaLayouts, p.ShowMetaEffective
	rowOpts := p.RowOpts
	y := rect.Y + 2 + row
	style := panelStyle.Styles.PanelRowFile
	text := ""
	entryIndex := state.ScrollOffset + row
	var spans []primitive.Span

	hasEntry := false
	var cur localfs.Entry
	selected := false
	fillCols := 0

	var subtreeMark bool
	var newFileTier panellist.NewFileMarkTier
	var renameMark bool
	var jobMark bool
	var jobStatus string
	var jobWrite bool
	var jobMarkGlyph rune
	var rowSuffix panellist.RowSuffix

	// Tree-mode gutter (ancestor guide lines + folder expander) is prepended before the
	// icon/name columns; every other column (size/date/permissions/git/marks) keeps using the
	// unmodified listTextWidth/listContentStart/nameWidth below.
	var treeConnector string
	var treeExpanded bool
	var treeLoading bool
	treeGutterWidth := 0

	if entry, _, ok := state.VisibleEntry(entryIndex); ok {
		hasEntry = true
		cur = entry
		if state.ListLayout == panel.ListLayoutTree {
			if depth, lastChild, ancestorHasNext, expanded, loading, trOK := state.TreeRowShape(entryIndex); trOK {
				treeConnector = panellist.TreeConnectorPrefix(depth, lastChild, ancestorHasNext, panelStyle.Styles)
				treeExpanded = expanded
				treeLoading = loading
				pw := len([]rune(treeConnector)) + panelIconStripCells
				// Narrow-panel fallback: hide the gutter rather than crush the name to nothing.
				if pw > 0 && pw < listTextWidth {
					treeGutterWidth = pw
				} else {
					treeConnector = ""
				}
			}
		}
		effTextWidth := listTextWidth - treeGutterWidth
		effNameWidth := max(1, nameWidth-treeGutterWidth)
		style, selected = panelRowStyle(entry, entryIndex, state, ctx, panelStyle.Styles)
		subtreeMark = entry.Type == localfs.EntryDirectory && effNameWidth > 2 && state.HasSelectionInSubtree(entry.Path)
		newFileTier = state.NewFileMarkTier(entry)
		renameMark = state.IsRenameMarked(entry)
		jobMark, jobStatus, jobWrite = EntryPathJobMarkStatus(entry.Path, display.JobMarks)
		if jobMark {
			jobMarkGlyph = panelStyle.Styles.SymbolFilelistJob()
		} else {
			jobMarkGlyph = 0
		}
		metaText := ""
		if showMetaEffective {
			metaText = MetaRowText(metaLayouts, entry.Path)
		}
		rowSuffix = panellist.NewRowSuffix(jobMarkGlyph, newFileTier, renameMark, subtreeMark, jobWrite)
		rowSuffix.Working = state.ShowLoadingGlyph && entry.Type == localfs.EntryDirectory && entry.Path == state.ListingPendingPath
		_, rowSuffix.Pinned = display.PinnedPaths[entry.Path]
		rowOpts.Suffix = rowSuffix
		text = formatEntry(entry, effTextWidth, rowOpts, panelStyle.Styles, display.Painter, metaText)
		nameWidth = effNameWidth
		listTextWidth = effTextWidth
		listContentStart += treeGutterWidth
		if display.ShowDiskUsage && display.Painter != nil && diskDenom > 0 {
			barMaxWidth := leftGutter + gitStrip + iconStrip + treeGutterWidth + nameWidth
			fillCols = diskUsageFillColumns(entryDiskUsageBytes(entry, true, display.Painter), diskDenom, barMaxWidth)
		}
	}

	blendCell := func(absCol int) tcell.Style {
		if !ctx.ChromeBlocked && fillCols > 0 && absCol >= 0 && absCol < fillCols {
			return mergeDiskUsageBackground(style, panelStyle.Styles.DiskUsageBarStyle(ctx.FileListActive, entryIndex == state.Cursor, selected))
		}
		return style
	}

	cursorIconKey := ""
	if hasEntry {
		cursorIconKey = panelCursorIconThemeKey(ctx.CursorRowActive, ctx.ChromeBlocked, entryIndex, state.Cursor, selected, ctx.CursorRowActive && state.FilterUniqueMatch())
	}
	if hasEntry {
		spans = matchSpans(cur, listTextWidth, state.MatchRanges(entryIndex), entryIndex == state.Cursor, panelStyle.Styles, rowOpts, func(di int) tcell.Style {
			return blendCell(di + leftGutter + gitStrip + iconStrip + treeGutterWidth)
		})
		if suffixSpans := panellist.ListingSuffixSpans(cur, nameWidth, display.ShowIcons, rowSuffix, jobStatus, panelStyle.Styles, ctx.ChromeBlocked, cursorIconKey, func(di int) tcell.Style {
			return blendCell(di + leftGutter + gitStrip + iconStrip + treeGutterWidth)
		}); len(suffixSpans) > 0 {
			spans = append(suffixSpans, spans...)
		}
	}

	if display.ShowIcons && leftGutter > 0 {
		for i := 0; i < leftGutter; i++ {
			screen.SetContent(rect.X+1+i, y, ' ', nil, blendCell(i))
		}
	}
	iconKey := cursorIconKey
	var diskPending bool
	var diskExcluded bool
	if hasEntry {
		// Mount-boundary / godu-excluded folder icons and tints are disk-usage UI only.
		// DiskScanExcluded Stat's each directory row; on a network panel that runs even when the
		// user navigates the other column and dominates latency during background copy I/O.
		if display.ShowDiskUsage && display.Painter != nil && cur.Type == localfs.EntryDirectory {
			diskPending = display.Painter.PendingForPanel(cur.Path, ctx.PanelID)
			diskExcluded = display.Painter.DiskScanExcluded(cur.Path, display.DiskUsageDescendIntoMountPoints, state.ListingDevice, state.ListingDeviceValid, display.DiskUsageGoduIgnore)
		}
	}
	if showGit {
		drawPanelRowGitStrip(screen, p, panelRowPaintState{
			Y: y, HasEntry: hasEntry, Entry: cur, EntryIndex: entryIndex, Selected: selected,
			FillCols: fillCols, BlendCell: blendCell,
		})
	}
	if display.ShowIcons && state.ListLayout != panel.ListLayoutTree {
		drawPanelRowIconStrip(screen, p, panelRowPaintState{
			Y: y, HasEntry: hasEntry, Entry: cur, Style: style, BlendCell: blendCell,
			IconKey: iconKey, DiskPending: diskPending, DiskExcluded: diskExcluded,
		})
	}
	if treeGutterWidth > 0 {
		gutterX := listContentStart - treeGutterWidth
		_, rowBG, _ := style.Decompose()
		connectorStyle := panelStyle.Styles.PanelRowTreeConnector.Background(rowBG)
		connW := len([]rune(treeConnector))
		primitive.Text(screen, gutterX, y, connW, treeConnector, connectorStyle)
		if hasEntry {
			iconX := gutterX + connW
			iconStripStyle := blendCell(leftGutter + gitStrip + iconStrip + connW)
			iconCtx := panelIconStripContextFor(display, ctx, state, cur, iconKey, diskPending, diskExcluded)
			iconCtx.Folder.TreeExpanded = treeExpanded
			iconCtx.Folder.TreeLoading = treeLoading
			paintPanelIconStrip(screen, iconX, y, cur, iconStripStyle, panelStyle.Styles, iconCtx)
		}
	}
	primitive.StyledTextCellwise(screen, listContentStart, y, listTextWidth, text, func(ci int) tcell.Style {
		st := blendCell(leftGutter + gitStrip + iconStrip + treeGutterWidth + ci)
		if ci >= nameWidth {
			return panelStyle.Styles.PanelListingInfoStyle(st)
		}
		return st
	}, spans)
}

// panelRowStyle resolves the row's file/selected/cursor style, moved out of drawPanelRow's
// per-entry style-resolution block.
func panelRowStyle(entry localfs.Entry, entryIndex int, state panel.State, ctx PanelContext, styles theme.Theme) (style tcell.Style, selected bool) {
	style = styles.PanelListingEntryStyle(entry.Type, ctx.ChromeBlocked)
	selected = state.IsSelected(entry)
	if selected {
		style = styles.PanelListingSelectedStyle(ctx.ChromeBlocked)
	}
	if entryIndex == state.Cursor {
		style = styles.PanelListingCursorStyle(style, theme.PanelListingCursorOpts{
			ChromeBlocked:     ctx.ChromeBlocked,
			FileListActive:    ctx.CursorRowActive,
			Selected:          selected,
			FilterUniqueMatch: ctx.CursorRowActive && state.FilterUniqueMatch(),
		})
	}
	return style, selected
}

// panelRowPaintState carries the per-entry values drawPanelRow computes in its body that the
// git-strip and icon-strip paint helpers need alongside the shared panelRowParams.
type panelRowPaintState struct {
	Y            int
	HasEntry     bool
	Entry        localfs.Entry
	EntryIndex   int
	Selected     bool
	Style        tcell.Style
	FillCols     int
	BlendCell    func(int) tcell.Style
	IconKey      string
	DiskPending  bool
	DiskExcluded bool
}

// drawPanelRowGitStrip paints the git-status strip cell (or its blank filler when the row has
// no entry) for one row, moved out of drawPanelRow's git-strip block.
func drawPanelRowGitStrip(screen tcell.Screen, p panelRowParams, rp panelRowPaintState) {
	gitStyle := rp.BlendCell(p.LeftGutter)
	gitUnderUsage := !p.Ctx.ChromeBlocked && rp.FillCols > p.LeftGutter
	var gitUsageAccent tcell.Style
	if gitUnderUsage {
		gitUsageAccent = p.PanelStyle.Styles.DiskUsageBarStyle(p.Ctx.FileListActive, rp.EntryIndex == p.State.Cursor, rp.Selected)
	}
	if rp.HasEntry {
		paintGitColumn(screen, p.GitStart, rp.Y, panelGitCell(rp.Entry, p.State.GitByPath), gitStyle, p.PanelStyle.Styles, rp.EntryIndex == p.State.Cursor, gitUnderUsage, gitUsageAccent)
		paintGitRowTrailingGap(screen, p.GitStart, rp.Y, gitStyle)
	} else {
		paintGitStripBlank(screen, p.GitStart, rp.Y, gitStyle)
	}
}

// drawPanelRowIconStrip paints the devicon strip cell (or its blank filler when the row has no
// entry) for one row, moved out of drawPanelRow's icon-strip block.
func drawPanelRowIconStrip(screen tcell.Screen, p panelRowParams, rp panelRowPaintState) {
	if !rp.HasEntry {
		paintPanelIconStripBlank(screen, p.IconStart, rp.Y, rp.Style)
		return
	}
	iconStripStyle := rp.BlendCell(p.LeftGutter + p.GitStrip)
	paintPanelIconStrip(screen, p.IconStart, rp.Y, rp.Entry, iconStripStyle, p.PanelStyle.Styles,
		panelIconStripContextFor(p.Display, p.Ctx, p.State, rp.Entry, rp.IconKey, rp.DiskPending, rp.DiskExcluded))
}

// panelColumnLayout computes the leading-column (gutter/git/icon) widths and offsets for a
// panel listing, moved verbatim out of drawPanel's body.
func panelColumnLayout(rect Rect, state panel.State, display PanelDisplayConfig) panelColumnLayoutResult {
	interior := rect.Width - 2
	leftGutter := 0
	if display.ShowIcons {
		leftGutter = panelIconListLeadingGutter
	}
	iconStrip := 0
	if display.ShowIcons && state.ListLayout != panel.ListLayoutTree {
		iconStrip = panelIconStripCells
	}
	baseListWidth := interior - leftGutter - iconStrip
	nameOnlyDisplay := display.ShrunkenShowsNameOnly && baseListWidth < config.ShrunkenListingRowTextWidthThreshold
	showGit := panelListGitColumnActive(state, nameOnlyDisplay)
	gitStrip := 0
	if showGit {
		gitStrip = panelListGitStripWidth()
	}
	rowTextWidth := baseListWidth - gitStrip
	if rowTextWidth < 1 {
		rowTextWidth = 1
	}
	gitStart := rect.X + 1 + leftGutter
	iconStart := gitStart + gitStrip
	diskDenom := panelDiskUsageDenom(display.ShowDiskUsage, display.Painter, state.VisibleEntries())
	return panelColumnLayoutResult{
		LeftGutter: leftGutter, IconStrip: iconStrip, NameOnlyDisplay: nameOnlyDisplay,
		ShowGit: showGit, GitStrip: gitStrip, RowTextWidth: rowTextWidth,
		GitStart: gitStart, IconStart: iconStart, DiskDenom: diskDenom,
	}
}

// drawPanelCarousel paints the carousel-mode (parent/center/child column) listing for a panel
// when it fits the available width, and reports whether it did so. When it returns false, the
// caller falls through to the classic single-column listing (unchanged behavior from the
// original inline `if state.CarouselMode { if panelcarousel.LayoutFits(...) { ...; return } }`).
func drawPanelCarousel(screen tcell.Screen, p panelCarouselParams) bool {
	rect, state, panelStyle, ctx, display := p.Rect, p.State, p.PanelStyle, p.Ctx, p.Display
	bottomCtx, borderStyle, titleStyle := p.BottomCtx, p.BorderStyle, p.TitleStyle
	headerStyle, headerCarouselStyle := p.HeaderStyle, p.HeaderCarouselStyle
	chromeSurface, selectionSizeLabel, visibleRows := p.SurfaceStyle, p.SelectionSizeLabel, p.VisibleRows
	quickViewOn := ctx.QuickViewDriverPanelID >= 0
	filePreviewEligible := panelcarousel.FilePreviewEligible(rect, ctx.HideInactivePanel, display.CarouselLayout)
	showChildCol := panelcarousel.ShowChildPreviewColumn(state, quickViewOn, filePreviewEligible)
	if !panelcarousel.LayoutFits(rect, display.CarouselLayout, showChildCol) {
		return false
	}
	parent, _, child, childKind := panelcarousel.BuildColumns(state, visibleRows, quickViewOn, filePreviewEligible)
	measuredFitWidth := panelcarousel.MeasureFitColumnWidths(display.CarouselLayout, parent, state, display.ShowIcons, showChildCol, panelStyle.ScrollbarStyle, visibleRows)
	carouselDisk := panelcarousel.DiskUsage{
		Active:                 display.ShowDiskUsage,
		PanelID:                ctx.PanelID,
		ListingDevice:          state.ListingDevice,
		ListingDeviceValid:     state.ListingDeviceValid,
		DescendIntoMountPoints: display.DiskUsageDescendIntoMountPoints,
		GoduIgnore:             display.DiskUsageGoduIgnore,
		Source:                 display.Painter,
	}
	panelcarousel.DrawBody(screen, panelcarousel.BodyParams{
		Frame:                 rect,
		Center:                state,
		Parent:                parent,
		Child:                 child,
		Styles:                panelStyle.Styles,
		ChromeBlocked:         ctx.ChromeBlocked,
		FileListActive:        ctx.FileListActive,
		ShowIcons:             display.ShowIcons,
		HeaderStyle:           headerStyle,
		HeaderCarouselStyle:   headerCarouselStyle,
		SurfaceStyle:          chromeSurface,
		ShowChildColumn:       showChildCol,
		ChildPreviewKind:      childKind,
		DiskUsage:             carouselDisk,
		OtherPanelPath:        ctx.OtherPanelPath,
		ScrollbarStyle:        panelStyle.ScrollbarStyle,
		ScrollbarShowInactive: display.ScrollbarShowInactive,
		InactiveFrameStyle:    panelStyle.Styles.PanelInactiveFrame,
		Layout:                display.CarouselLayout,
		MeasuredFitWidth:      measuredFitWidth,
		JobMark: func(path string) (rune, string, bool, bool) {
			marked, st, write := EntryPathJobMarkStatus(path, display.JobMarks)
			if !marked {
				return 0, "", false, false
			}
			return panelStyle.Styles.SymbolFilelistJob(), st, write, true
		},
		PaintIcon: func(sc tcell.Screen, x, y int, entry localfs.Entry, rowStyle tcell.Style, cursorKey string, diskPending, diskExcluded bool) {
			paintPanelIconStrip(sc, x, y, entry, rowStyle, panelStyle.Styles,
				panelIconStripContextFor(display, ctx, state, entry, cursorKey, diskPending, diskExcluded))
		},
		NewFileMark: func(entry localfs.Entry) panellist.NewFileMarkTier {
			return state.NewFileMarkTier(entry)
		},
		RenameMark: func(entry localfs.Entry) bool {
			return state.IsRenameMarked(entry)
		},
	})
	paintCarouselFilePreview := display.CarouselFilePreview.Open && showChildCol &&
		(childKind == panelcarousel.ChildPreviewFile ||
			ctx.FileListActive && (state.Filter.Active || state.Filter.Editing))
	if paintCarouselFilePreview {
		if previewRect, ok := panelcarousel.ChildPreviewPaintRect(rect, showChildCol, display.CarouselLayout, measuredFitWidth); ok {
			// The child preview's own rect stops one column short of the panel's real
			// border (a blank margin column sits between them) — point the scrollbar at
			// the border column itself, matching the plain file list's scrollbar position,
			// and use the panel's own (non-Chroma-tinted) border color for the rail so it
			// matches that border column's usual color, same as the file list's scrollbar.
			drawFilePreviewPanel(screen, Rect(previewRect), display.CarouselFilePreview, panelStyle.Styles, ctx.ChromeBlocked, false, false, true, false, state.PathString(), display.UserHomeDir, panelStyle.ScrollbarStyle, rect.X+rect.Width-1, borderStyle)
		}
	}
	if !showChildCol {
		drawPanelListScrollbar(screen, rect,
			panelScrollPos{ListTopY: rect.Y + 2, Visible: visibleRows, Total: state.VisibleEntryCount(), Offset: state.ScrollOffset},
			panelStyle.ScrollbarStyle, panelScrollbarShow(ctx.FileListActive, display.ScrollbarShowInactive),
			ctx.FileListActive, ctx.ChromeBlocked, borderStyle, panelStyle.Styles)
	}
	if selectionSizeLabel != "" {
		drawPanelBottomSelectionSize(screen, rect, ctx.PanelID, bottomCtx)
	} else {
		drawPanelCursorNameHintForState(screen, rect, ctx.PanelID, state, bottomCtx, ctx.FileListActive, ctx.ChromeBlocked, titleStyle, display.ShowIcons, panelcarousel.CenterNameWidth(rect, display.CarouselLayout, state, display.ShowIcons, showChildCol, panelStyle.ScrollbarStyle, visibleRows, measuredFitWidth), display.JobMarks, ctx.CursorNameHintFallbackOut, ctx.CursorNameHintPinnedOut)
	}
	return true
}

const gapBeforePanelTitleEnd = 2

// panelTitleStyles groups the three tcell.Style values needed by paintPanelTopTitleRow.
type panelTitleStyles struct {
	Path, End, Border tcell.Style
}

// paintPanelTopTitleRow paints the panel top border path on the start and an optional end label
// (volume overview with decorative dashes, or a plain title-styled suffix such as a filename).
func paintPanelTopTitleRow(screen tcell.Screen, titleX, innerRight, contentCols, y int,
	panelPath, userHomeDir string, ts panelTitleStyles, endLabel string, volumeDecorated bool) {
	pathSlotCols := contentCols
	endRunes := utf8.RuneCountInString(endLabel)
	// Plain end labels (filename / QV dir basename) leave one frame-dash before the corner;
	// volume labels already include a trailer " ─".
	endRightMargin := 0
	if !volumeDecorated {
		endRightMargin = 1
	}
	showEnd := endLabel != "" && endRunes > 0 && contentCols >= endRunes+gapBeforePanelTitleEnd+endRightMargin+3
	endStartX := 0
	if showEnd {
		endStartX = innerRight - endRunes + 1 - endRightMargin
		pathSlotCols = endStartX - titleX - gapBeforePanelTitleEnd
		if pathSlotCols < 3 {
			showEnd = false
			pathSlotCols = contentCols
		}
	}
	pathMax := pathSlotCols - 2
	if pathMax < 0 {
		pathMax = 0
	}
	left := " " + PanelTitlePath(panelPath, userHomeDir, pathMax) + " "
	primitive.TextOverlay(screen, titleX, y, pathSlotCols, left, ts.Path)
	if !showEnd {
		return
	}
	if volumeDecorated {
		leaderRunes := utf8.RuneCountInString(panelVolumeTitleLeader)
		trailerRunes := utf8.RuneCountInString(panelVolumeTitleTrailer)
		primitive.TextOverlay(screen, endStartX, y, leaderRunes, panelVolumeTitleLeader, ts.Border)
		contentX := endStartX + leaderRunes
		contentLen := endRunes - leaderRunes - trailerRunes
		contentText := string([]rune(endLabel)[leaderRunes : leaderRunes+contentLen])
		primitive.TextOverlay(screen, contentX, y, contentLen, contentText, ts.End)
		primitive.TextOverlay(screen, endStartX+endRunes-trailerRunes, y, trailerRunes, panelVolumeTitleTrailer, ts.Border)
		return
	}
	primitive.TextOverlay(screen, endStartX, y, endRunes, endLabel, ts.End)
}

const (
	panelGitignoreChromeName   = "Gitignore"
	panelGitignoreChromePadded = " " + panelGitignoreChromeName + " "
)

// panelCursorIconThemeKey is the semantic style key for Theme.PanelFileIconFG on the cursor row; empty otherwise.
func panelCursorIconThemeKey(fileListActive, chromeBlocked bool, entryIndex, cursor int, selected, filterUniqueMatch bool) string {
	if entryIndex != cursor {
		return ""
	}
	if chromeBlocked {
		if selected {
			return "panel.blocked.row.cursor.selected"
		}
		return "panel.blocked.row.cursor"
	}
	if fileListActive {
		if filterUniqueMatch {
			return "panel.active.row.cursor.unique"
		}
		if selected {
			return "panel.active.row.cursor.selected"
		}
		return "panel.active.row.cursor"
	}
	if selected {
		return "panel.inactive.row.cursor.selected"
	}
	return "panel.inactive.row.cursor"
}

func panelListHeader(rowTextWidth int, state panel.State, showIcons bool, showMeta bool, metaLayouts []MetaColumnLayout, nameOnly, showGit bool) string {
	if nameOnly {
		nameTitle, sizeTitle, thirdTitle := state.ListColumnTitles(showIcons)
		title := panelListHeaderTitleWithSortArrow(nameTitle, sizeTitle, thirdTitle)
		title = truncateHeaderRunes(rowTextWidth, title)
		return fmt.Sprintf("%-*s", rowTextWidth, title)
	}
	listFmt := panel.EffectiveListFormat(state.ListFormat)
	tw := panelListThirdColumnWidth(listFmt, false)
	metaTotalW := 0
	for i, lay := range metaLayouts {
		if i > 0 {
			metaTotalW += 2
		}
		metaTotalW += lay.Width
	}
	// rowTextWidth already has the git strip excluded; pass false to avoid double-subtracting.
	nameWidth := panelListNameWidth(rowTextWidth, listFmt, false, false)
	if showMeta {
		nameWidth = panelListNameWidthWithMeta(rowTextWidth, metaTotalW, listFmt, false, false)
	}
	nameTitle, sizeTitle, thirdTitle := state.ListColumnTitles(showIcons)
	nameTitle = truncateHeaderRunes(nameWidth, nameTitle)
	sizeTitle = truncateHeaderRunes(panelListSizeCells, sizeTitle)
	metaHdr := MetaHeaderText(metaLayouts)
	if tw == 0 {
		if showMeta {
			return fmt.Sprintf("%-*s  %s %*s", nameWidth, nameTitle, metaHdr, panelListSizeCells, sizeTitle)
		}
		return fmt.Sprintf("%-*s %*s", nameWidth, nameTitle, panelListSizeCells, sizeTitle)
	}
	thirdTitle = truncateHeaderRunes(tw, thirdTitle)
	if showMeta {
		return fmt.Sprintf("%-*s  %s %*s  %-*s", nameWidth, nameTitle, metaHdr, panelListSizeCells, sizeTitle, tw, thirdTitle)
	}
	return fmt.Sprintf("%-*s %*s  %-*s", nameWidth, nameTitle, panelListSizeCells, sizeTitle, tw, thirdTitle)
}

// panelRowOpts holds display options shared by formatEntry and matchSpans.
type panelRowOpts struct {
	ShowIcons bool
	Suffix    panellist.RowSuffix
	ShowMeta  bool
	MetaColW  int
	ListFmt   panel.ListFormat
	NameOnly  bool
	ShowGit   bool
}

func formatEntry(entry localfs.Entry, width int, opts panelRowOpts, styles theme.Theme, painter DiskUsagePainter, metaText string) string {
	listFmt := panel.EffectiveListFormat(opts.ListFmt)
	tw := panelListThirdColumnWidth(listFmt, opts.NameOnly)
	nameWidth := panelListNameWidth(width, listFmt, opts.NameOnly, false)
	if opts.ShowMeta {
		nameWidth = panelListNameWidthWithMeta(width, opts.MetaColW, listFmt, opts.NameOnly, false)
	}
	display := panellist.EntryDisplayRunes(entry, nameWidth, opts.ShowIcons, opts.Suffix, styles)
	name := string(panellist.RunesFromDisplay(display))
	if opts.NameOnly {
		return fmt.Sprintf("%-*s", width, name)
	}
	if tw == 0 {
		if opts.ShowMeta {
			metaPadded := padMetaLineToWidth(metaText, opts.MetaColW)
			return fmt.Sprintf("%-*s  %s %*s", nameWidth, name, metaPadded, panelListSizeCells, formatListedSize(entry, painter))
		}
		return fmt.Sprintf("%-*s %*s", nameWidth, name, panelListSizeCells, formatListedSize(entry, painter))
	}
	var third string
	switch listFmt {
	case panel.ListFormatPerm:
		third = formatListedPerm(entry)
	default:
		third = formatTime(entry.ModifiedAt)
	}
	if opts.ShowMeta {
		metaPadded := padMetaLineToWidth(metaText, opts.MetaColW)
		return fmt.Sprintf("%-*s  %s %*s  %-*s", nameWidth, name, metaPadded, panelListSizeCells, formatListedSize(entry, painter), tw, third)
	}
	return fmt.Sprintf("%-*s %*s  %-*s", nameWidth, name, panelListSizeCells, formatListedSize(entry, painter), tw, third)
}

func matchSpans(entry localfs.Entry, rowWidth int, ranges []search.Range, highlightCursor bool, styles theme.Theme, opts panelRowOpts, nameBGAt func(displayIndex int) tcell.Style) []primitive.Span {
	if len(ranges) == 0 {
		return nil
	}
	listFmt := panel.EffectiveListFormat(opts.ListFmt)
	// rowWidth already has the git strip excluded; pass false to avoid double-subtracting.
	nameWidth := panelListNameWidth(rowWidth, listFmt, opts.NameOnly, false)
	if opts.ShowMeta {
		nameWidth = panelListNameWidthWithMeta(rowWidth, opts.MetaColW, listFmt, opts.NameOnly, false)
	}
	display := panellist.EntryDisplayRunes(entry, nameWidth, opts.ShowIcons, opts.Suffix, styles)
	matchStyle := styles.FuzzyHighlight
	if highlightCursor {
		matchStyle = styles.FuzzyHighlightCursor
	}
	spans := make([]primitive.Span, 0, len(ranges))
	for displayIndex, dr := range display {
		if dr.NameIdx < 0 || !rangeContains(ranges, dr.NameIdx) {
			continue
		}
		_, background, _ := nameBGAt(displayIndex).Decompose()
		ms := matchStyle.Background(background)
		spans = append(spans, primitive.Span{
			Start: displayIndex,
			End:   displayIndex + 1,
			Style: ms,
		})
	}
	return spans
}

func rangeContains(ranges []search.Range, index int) bool {
	for _, r := range ranges {
		if index >= r.Start && index < r.End {
			return true
		}
	}
	return false
}

const (
	panelVolumeTitleLeader  = "──── "
	panelVolumeTitleTrailer = " ─"
	panelVolumeTitleMaxUnit = 8 // same compact binary scaling as listing sizes (K/M/G/…)
)

func panelVolumeFreeSpaceTitle(ok bool, avail, total uint64) string {
	if !ok || total == 0 {
		return ""
	}
	pct := int(avail * 100 / total)
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}
	freeStr, totalStr := panelVolumePairSizes(avail, total, panelVolumeTitleMaxUnit)
	return fmt.Sprintf("%s%s / %s (%d%%)%s", panelVolumeTitleLeader, freeStr, totalStr, pct, panelVolumeTitleTrailer)
}

// panelVolumePairSizes formats free/total with the same unit suffix when possible so compact tiers align (e.g. 28G / 1817G).
func panelVolumePairSizes(avail, total uint64, maxW int) (freeStr, totalStr string) {
	a := uint64ToListingSizeInt64(avail)
	b := uint64ToListingSizeInt64(total)
	ia := byteSizeCompactSuffixIdx(a)
	ib := byteSizeCompactSuffixIdx(b)
	if ia < 0 || ib < 0 {
		return formatByteSizeCompact(a, maxW), formatByteSizeCompact(b, maxW)
	}
	use := ia
	if ib < use {
		use = ib
	}
	div := tierDivisorFromSuffixIdx(use)
	sfx := byteSizeCompactSuffixRune(use)
	va := float64(a) / float64(div)
	vb := float64(b) / float64(div)
	return formatHumanScaled(va, sfx, maxW), formatHumanScaled(vb, sfx, maxW)
}

func byteSizeCompactSuffixIdx(n int64) int {
	const KiB = int64(1024)
	if n < KiB {
		return -1
	}
	v := float64(n) / float64(KiB)
	sfxIdx := 0
	suffixes := byteCompactSuffixes[:]
	for v >= 1024 && sfxIdx < len(suffixes)-1 {
		v /= 1024
		sfxIdx++
	}
	return sfxIdx
}

func tierDivisorFromSuffixIdx(sfxIdx int) uint64 {
	const KiB = uint64(1024)
	if sfxIdx < 0 {
		return 1
	}
	d := KiB
	for range sfxIdx {
		d *= 1024
	}
	return d
}

func byteSizeCompactSuffixRune(sfxIdx int) byte {
	if sfxIdx < 0 || sfxIdx >= len(byteCompactSuffixes) {
		return byteCompactSuffixes[0]
	}
	return byteCompactSuffixes[sfxIdx]
}

// byteCompactSuffixes matches formatByteSizeListed / formatByteSizeCompact scaling.
var byteCompactSuffixes = [...]byte{'K', 'M', 'G', 'T', 'P', 'E'}

func uint64ToListingSizeInt64(u uint64) int64 {
	maxI := uint64(math.MaxInt64)
	if u > maxI {
		return math.MaxInt64
	}
	return int64(u)
}

func formatListedSize(entry localfs.Entry, painter DiskUsagePainter) string {
	if entry.Type == localfs.EntryDirectory {
		if painter != nil {
			if sz, ok := painter.ByteSize(entry.Path); ok {
				return formatByteSizeListed(sz)
			}
		}
		return ""
	}
	return formatByteSizeListed(entry.Size)
}

// formatByteSizeListed renders a short right-aligned size for the panel column (see panelListSizeCells).
func formatByteSizeListed(n int64) string {
	return formatByteSizeCompact(n, panelListSizeCells)
}

// formatByteSizeCompact renders n using the same binary scaling as the panel size column (KiB steps, K/M/G/… suffix).
func formatByteSizeCompact(n int64, maxW int) string {
	const KiB = int64(1024)
	if maxW < 1 {
		return ""
	}
	if n < 0 {
		n = 0
	}
	if n < KiB {
		s := strconv.FormatInt(n, 10)
		if len(s) > maxW {
			return s[:maxW]
		}
		return s
	}
	v := float64(n)
	suffixes := byteCompactSuffixes[:]
	v /= float64(KiB)
	sfxIdx := 0
	for v >= 1024 && sfxIdx < len(suffixes)-1 {
		v /= 1024
		sfxIdx++
	}
	return formatHumanScaled(v, suffixes[sfxIdx], maxW)
}

func formatHumanScaled(v float64, sfx byte, maxW int) string {
	if v >= 10 || math.Abs(v-math.Round(v)) < 1e-3 {
		s := fmt.Sprintf("%.0f%c", v, sfx)
		if len(s) <= maxW {
			return s
		}
	}
	s := fmt.Sprintf("%.1f%c", v, sfx)
	if len(s) <= maxW {
		return s
	}
	s = fmt.Sprintf("%.0f%c", v, sfx)
	if len(s) <= maxW {
		return s
	}
	if len(s) > maxW {
		return s[:maxW]
	}
	return s
}

// FormatByteSize renders a byte count with binary prefixes (B, KiB, MiB, GiB, TiB) for dialogs and labels.
func FormatByteSize(n int64) string {
	if n < 0 {
		n = 0
	}
	const (
		KiB = int64(1024)
		MiB = KiB * 1024
		GiB = MiB * 1024
		TiB = GiB * 1024
	)
	switch {
	case n < KiB:
		return fmt.Sprintf("%d B", n)
	case n < MiB:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(KiB))
	case n < GiB:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(MiB))
	case n < TiB:
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(GiB))
	default:
		return fmt.Sprintf("%.1f TiB", float64(n)/float64(TiB))
	}
}

func formatListedPerm(entry localfs.Entry) string {
	s := entry.Mode.String()
	if len(s) > panelListPermCells {
		s = s[:panelListPermCells]
	}
	return fmt.Sprintf("%-*s", panelListPermCells, s)
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04")
}
