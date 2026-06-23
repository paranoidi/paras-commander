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

func drawPanel(screen tcell.Screen, rect Rect, state panel.State, fileListActive bool, chromeBlocked bool, styles theme.Theme, showIcons bool, userHomeDir string, painter DiskUsagePainter, diskUsageDescendIntoMountPoints bool, diskUsageGoduIgnore func(string) bool, showDiskUsage bool, panelID int, jobMarks []JobPathMark, syncDriverPanelID, quickViewDriverPanelID int, metaColumns []MetaColumnState, shrunkenShowsNameOnly bool, selectionsBottomHint bool, hideInactivePanel bool, activePanel int, otherPanelPath string, showSelectionSizeOnBottom bool, scrollbarStyle uiscrollbar.Style, scrollbarShowInactive bool, carouselLayout panelcarousel.Layout, carouselFilePreview FilePreviewState, previewChromaStyle string, splitOrientation SplitOrientation) {
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	chrome := styles.PanelChrome(fileListActive, chromeBlocked)
	borderStyle := chrome.Frame
	titleStyle := chrome.Title
	headerStyle := chrome.Header
	headerCarouselStyle := chrome.HeaderCarousel

	primitive.Box(screen, primitive.Rect(rect), borderStyle)
	var selectionSizeLabel string
	if showSelectionSizeOnBottom && state.SelectedPathCount() > 0 {
		selectionSizeLabel, _ = SelectionSizeLabel(
			&state,
			state.Path.IsRemote(),
			painter,
			diskUsageDescendIntoMountPoints,
			diskUsageGoduIgnore,
			styles.SymbolWorking(),
		)
	}
	bottomCtx := panelBottomIndicatorContextForRect(
		rect, panelID, state, selectionsBottomHint,
		syncDriverPanelID, quickViewDriverPanelID,
		hideInactivePanel, activePanel, otherPanelPath, userHomeDir,
		fileListActive, chromeBlocked,
		borderStyle, styles,
		selectionSizeLabel,
		splitOrientation,
	)
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
	if fileListActive && (state.Filter.Active || state.Filter.Editing) {
		inputStyle := styles.FuzzyInput
		if state.Filter.Active && !state.FilterHasMatches() {
			inputStyle = styles.FuzzyInputNomatch
		}
		primitive.Text(screen, titleX, rect.Y, contentCols, "> "+state.Filter.Query, inputStyle)
	} else {
		volumeLabel := panelVolumeFreeSpaceTitle(state.VolumeSpaceOK, state.VolumeAvailBytes, state.VolumeTotalBytes)
		paintPanelTopTitleRow(screen, titleX, innerRight, contentCols, rect.Y,
			state.PathString(), userHomeDir, titleStyle,
			volumeLabel, chrome.DiskUsageOverview, borderStyle, true)
	}

	visibleRows := PanelListRows(rect)
	if visibleRows == 0 {
		return
	}

	if state.CarouselMode {
		quickViewOn := quickViewDriverPanelID >= 0
		filePreviewEligible := panelcarousel.FilePreviewEligible(rect, hideInactivePanel, carouselLayout)
		showChildCol := panelcarousel.ShowChildPreviewColumn(state, quickViewOn, filePreviewEligible)
		if panelcarousel.LayoutFits(rect, carouselLayout, showChildCol) {
			parent, _, child, childKind := panelcarousel.BuildColumns(state, visibleRows, quickViewOn, filePreviewEligible)
			carouselDisk := panelcarousel.DiskUsage{
				Active:                 showDiskUsage,
				PanelID:                panelID,
				ListingDevice:          state.ListingDevice,
				ListingDeviceValid:     state.ListingDeviceValid,
				DescendIntoMountPoints: diskUsageDescendIntoMountPoints,
				GoduIgnore:             diskUsageGoduIgnore,
				Source:                 painter,
			}
			panelcarousel.DrawBody(screen, panelcarousel.BodyParams{
				Frame:                 rect,
				Center:                state,
				Parent:                parent,
				Child:                 child,
				Styles:                styles,
				ChromeBlocked:         chromeBlocked,
				FileListActive:        fileListActive,
				ShowIcons:             showIcons,
				HeaderStyle:           headerStyle,
				HeaderCarouselStyle:   headerCarouselStyle,
				SurfaceStyle:          chrome.Surface,
				ShowChildColumn:       showChildCol,
				ChildPreviewKind:      childKind,
				DiskUsage:             carouselDisk,
				OtherPanelPath:        otherPanelPath,
				ScrollbarStyle:        scrollbarStyle,
				ScrollbarShowInactive: scrollbarShowInactive,
				InactiveFrameStyle:    styles.PanelInactiveFrame,
				Layout:                carouselLayout,
				JobMark: func(path string) (rune, string, bool) {
					marked, st := EntryPathJobMarkStatus(path, jobMarks)
					if !marked {
						return 0, "", false
					}
					glyphStr := styles.SymbolJobsList(st)
					if glyphStr == "" {
						return 0, "", false
					}
					r, _ := utf8.DecodeRuneInString(glyphStr)
					return r, st, true
				},
				PaintIcon: func(sc tcell.Screen, x, y int, entry localfs.Entry, rowStyle tcell.Style, cursorKey string, diskPending, diskExcluded bool) {
					paintPanelIconStrip(sc, x, y, entry, rowStyle, styles, PanelIconStripContext{
						CursorStyleKey: cursorKey,
						ChromeBlocked:  chromeBlocked,
						Folder: panellist.FolderIconContext{
							OtherPanelPath:         otherPanelPath,
							DescendIntoMountPoints: diskUsageDescendIntoMountPoints,
							ListingDev:             state.ListingDevice,
							ListingDevValid:        state.ListingDeviceValid,
							DiskPending:            diskPending,
							DiskExcluded:           diskExcluded,
							DiskUsageChrome:        showDiskUsage,
						},
					})
				},
				NewFileMark: func(entry localfs.Entry) panellist.NewFileMarkTier {
					return state.NewFileMarkTier(entry)
				},
				RenameMark: func(entry localfs.Entry) bool {
					return state.IsRenameMarked(entry)
				},
			})
			paintCarouselFilePreview := carouselFilePreview.Open && showChildCol &&
				(childKind == panelcarousel.ChildPreviewFile ||
					fileListActive && (state.Filter.Active || state.Filter.Editing))
			if paintCarouselFilePreview {
				if previewRect, ok := panelcarousel.ChildPreviewPaintRect(rect, showChildCol, carouselLayout); ok {
					drawFilePreviewPanel(screen, Rect(previewRect), carouselFilePreview, styles, chromeBlocked, false, false, true, state.PathString(), userHomeDir, previewChromaStyle)
				}
			}
			if !showChildCol {
				drawPanelListScrollbar(screen, rect, rect.Y+2, visibleRows, state.VisibleEntryCount(), state.ScrollOffset,
					scrollbarStyle, panelScrollbarShow(fileListActive, scrollbarShowInactive),
					fileListActive, chromeBlocked, borderStyle, styles)
			}
			if selectionSizeLabel != "" {
				drawPanelBottomSelectionSize(screen, rect, panelID, bottomCtx)
			} else {
				drawPanelCursorNameHintForState(screen, rect, panelID, state, bottomCtx, fileListActive, chromeBlocked, titleStyle, showIcons, panelcarousel.CenterNameWidth(rect, carouselLayout, state, showIcons, showChildCol, scrollbarStyle, visibleRows), jobMarks)
			}
			return
		}
	}

	interior := rect.Width - 2
	leftGutter := 0
	if showIcons {
		leftGutter = panelIconListLeadingGutter
	}
	iconStrip := 0
	if showIcons {
		iconStrip = panelIconStripCells
	}
	baseListWidth := interior - leftGutter - iconStrip
	nameOnlyDisplay := shrunkenShowsNameOnly && baseListWidth < config.ShrunkenListingRowTextWidthThreshold
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
	fullRowCells := leftGutter + gitStrip + iconStrip + rowTextWidth
	diskDenom := panelDiskUsageDenom(showDiskUsage, painter, state.Entries)

	metaLayouts, metaTotalW := LayoutMetaColumns(metaColumns)
	showMeta := len(metaLayouts) > 0
	showMetaEffective := showMeta && !nameOnlyDisplay
	listTextWidth := rowTextWidth
	header := panelListHeader(listTextWidth, state, showIcons, showMetaEffective, metaLayouts, nameOnlyDisplay, showGit)
	headerY := rect.Y + 1
	if leftGutter > 0 {
		for i := 0; i < leftGutter; i++ {
			screen.SetContent(rect.X+1+i, headerY, ' ', nil, headerStyle)
		}
	}
	if showGit {
		paintGitHeader(screen, gitStart, headerY, headerStyle, styles)
	}
	if showIcons {
		paintPanelIconStripBlank(screen, iconStart, headerY, headerStyle)
	}
	listContentStart := iconStart + iconStrip
	primitive.Text(screen, listContentStart, headerY, listTextWidth, header, headerStyle)

	listFmt := panel.EffectiveListFormat(state.ListFormat)
	// listTextWidth already has the git strip excluded; pass false to avoid double-subtracting.
	nameWidth := panelListNameWidth(listTextWidth, listFmt, nameOnlyDisplay, false)
	if showMetaEffective {
		nameWidth = panelListNameWidthWithMeta(listTextWidth, metaTotalW, listFmt, nameOnlyDisplay, false)
	}
	for row := 0; row < visibleRows; row++ {
		y := rect.Y + 2 + row
		style := styles.PanelRowFile
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
		var jobMarkGlyph rune
		var rowSuffix panellist.RowSuffix

		if entry, _, ok := state.VisibleEntry(entryIndex); ok {
			hasEntry = true
			cur = entry
			style = styles.PanelListingEntryStyle(entry.Type, chromeBlocked)
			selected = state.IsSelected(entry)
			if selected {
				style = styles.PanelListingSelectedStyle(chromeBlocked)
			}
			if entryIndex == state.Cursor {
				style = styles.PanelListingCursorStyle(theme.PanelListingCursorOpts{
					ChromeBlocked:     chromeBlocked,
					FileListActive:    fileListActive,
					Selected:          selected,
					FilterUniqueMatch: fileListActive && state.FilterUniqueMatch(),
				})
			}
			subtreeMark = entry.Type == localfs.EntryDirectory && nameWidth > 2 && state.HasSelectionInSubtree(entry.Path)
			newFileTier = state.NewFileMarkTier(entry)
			renameMark = state.IsRenameMarked(entry)
			jobMark, jobStatus = EntryPathJobMarkStatus(entry.Path, jobMarks)
			if jobMark {
				glyphStr := styles.SymbolJobsList(jobStatus)
				if glyphStr != "" {
					jobMarkGlyph, _ = utf8.DecodeRuneInString(glyphStr)
				} else {
					jobMarkGlyph = 0
				}
			} else {
				jobMarkGlyph = 0
			}
			metaText := ""
			if showMetaEffective {
				metaText = MetaRowText(metaLayouts, entry.Path)
			}
			rowSuffix = panellist.RowSuffix{
				JobGlyph:         jobMarkGlyph,
				NewFileTier:      newFileTier,
				RenameMark:       renameMark,
				SubtreeSelection: subtreeMark,
			}
			text = formatEntry(entry, listTextWidth, showIcons, rowSuffix, styles, painter, showMetaEffective, metaTotalW, metaText, listFmt, nameOnlyDisplay)
			if showDiskUsage && painter != nil && diskDenom > 0 {
				fillCols = diskUsageFillColumns(entryDiskUsageBytes(entry, true, painter), diskDenom, fullRowCells)
			}
		}

		blendCell := func(absCol int) tcell.Style {
			if !chromeBlocked && fillCols > 0 && absCol >= 0 && absCol < fillCols {
				return mergeDiskUsageBackground(style, styles.DiskUsageBarStyle(fileListActive, entryIndex == state.Cursor, selected))
			}
			return style
		}

		cursorIconKey := ""
		if hasEntry {
			cursorIconKey = panelCursorIconThemeKey(fileListActive, chromeBlocked, entryIndex, state.Cursor, selected, fileListActive && state.FilterUniqueMatch())
		}
		if hasEntry {
			spans = matchSpans(cur, listTextWidth, state.MatchRanges(entryIndex), entryIndex == state.Cursor, styles, showIcons, rowSuffix, showMetaEffective, metaTotalW, listFmt, nameOnlyDisplay, showGit, func(di int) tcell.Style {
				return blendCell(di + leftGutter + gitStrip + iconStrip)
			})
			if suffixSpans := panellist.ListingSuffixSpans(cur, nameWidth, showIcons, rowSuffix, jobStatus, styles, chromeBlocked, cursorIconKey, func(di int) tcell.Style {
				return blendCell(di + leftGutter + gitStrip + iconStrip)
			}); len(suffixSpans) > 0 {
				spans = append(suffixSpans, spans...)
			}
		}

		if showIcons && leftGutter > 0 {
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
			if showDiskUsage && painter != nil && cur.Type == localfs.EntryDirectory {
				diskPending = painter.PendingForPanel(cur.Path, panelID)
				diskExcluded = painter.DiskScanExcluded(cur.Path, diskUsageDescendIntoMountPoints, state.ListingDevice, state.ListingDeviceValid, diskUsageGoduIgnore)
			}
		}
		if showGit {
			gitStyle := blendCell(leftGutter)
			gitUnderUsage := !chromeBlocked && fillCols > leftGutter
			var gitUsageAccent tcell.Style
			if gitUnderUsage {
				gitUsageAccent = styles.DiskUsageBarStyle(fileListActive, entryIndex == state.Cursor, selected)
			}
			if hasEntry {
				paintGitColumn(screen, gitStart, y, panelGitCell(cur, state.GitByPath), gitStyle, styles, entryIndex == state.Cursor, gitUnderUsage, gitUsageAccent)
				paintGitRowTrailingGap(screen, gitStart, y, gitStyle)
			} else {
				paintGitStripBlank(screen, gitStart, y, gitStyle)
			}
		}
		if showIcons {
			if hasEntry {
				iconStripStyle := blendCell(leftGutter + gitStrip)
				paintPanelIconStrip(screen, iconStart, y, cur, iconStripStyle, styles, PanelIconStripContext{
					CursorStyleKey: iconKey,
					ChromeBlocked:  chromeBlocked,
					Folder: panellist.FolderIconContext{
						OtherPanelPath:         otherPanelPath,
						DescendIntoMountPoints: diskUsageDescendIntoMountPoints,
						ListingDev:             state.ListingDevice,
						ListingDevValid:        state.ListingDeviceValid,
						DiskPending:            diskPending,
						DiskExcluded:           diskExcluded,
						DiskUsageChrome:        showDiskUsage,
					},
				})
			} else {
				paintPanelIconStripBlank(screen, iconStart, y, style)
			}
		}
		primitive.StyledTextCellwise(screen, listContentStart, y, listTextWidth, text, func(ci int) tcell.Style {
			return blendCell(leftGutter + gitStrip + iconStrip + ci)
		}, spans)
	}

	drawPanelListScrollbar(screen, rect, rect.Y+2, visibleRows, state.VisibleEntryCount(), state.ScrollOffset,
		scrollbarStyle, panelScrollbarShow(fileListActive, scrollbarShowInactive),
		fileListActive, chromeBlocked, borderStyle, styles)

	if selectionSizeLabel != "" {
		drawPanelBottomSelectionSize(screen, rect, panelID, bottomCtx)
	} else {
		drawPanelCursorNameHintForState(screen, rect, panelID, state, bottomCtx, fileListActive, chromeBlocked, titleStyle, showIcons, nameWidth, jobMarks)
	}
}

const gapBeforePanelTitleEnd = 2

// paintPanelTopTitleRow paints the panel top border path on the start and an optional end label
// (volume overview with decorative dashes, or a plain title-styled suffix such as a filename).
func paintPanelTopTitleRow(screen tcell.Screen, titleX, innerRight, contentCols, y int,
	panelPath, userHomeDir string, pathStyle tcell.Style, endLabel string, endStyle, borderStyle tcell.Style, volumeDecorated bool) {
	pathSlotCols := contentCols
	endRunes := utf8.RuneCountInString(endLabel)
	showEnd := endLabel != "" && endRunes > 0 && contentCols >= endRunes+gapBeforePanelTitleEnd+3
	endStartX := 0
	if showEnd {
		endStartX = innerRight - endRunes + 1
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
	primitive.TextOverlay(screen, titleX, y, pathSlotCols, left, pathStyle)
	if !showEnd {
		return
	}
	if volumeDecorated {
		leaderRunes := utf8.RuneCountInString(panelVolumeTitleLeader)
		trailerRunes := utf8.RuneCountInString(panelVolumeTitleTrailer)
		primitive.TextOverlay(screen, endStartX, y, leaderRunes, panelVolumeTitleLeader, borderStyle)
		contentX := endStartX + leaderRunes
		contentLen := endRunes - leaderRunes - trailerRunes
		contentText := string([]rune(endLabel)[leaderRunes : leaderRunes+contentLen])
		primitive.TextOverlay(screen, contentX, y, contentLen, contentText, endStyle)
		primitive.TextOverlay(screen, endStartX+endRunes-trailerRunes, y, trailerRunes, panelVolumeTitleTrailer, borderStyle)
		return
	}
	primitive.TextOverlay(screen, endStartX, y, endRunes, endLabel, endStyle)
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

func formatEntry(entry localfs.Entry, width int, showFileIcons bool, suffix panellist.RowSuffix, styles theme.Theme, painter DiskUsagePainter, showMeta bool, metaColW int, metaText string, listFmt panel.ListFormat, nameOnly bool) string {
	listFmt = panel.EffectiveListFormat(listFmt)
	tw := panelListThirdColumnWidth(listFmt, nameOnly)
	nameWidth := panelListNameWidth(width, listFmt, nameOnly, false)
	if showMeta {
		nameWidth = panelListNameWidthWithMeta(width, metaColW, listFmt, nameOnly, false)
	}
	display := panellist.EntryDisplayRunes(entry, nameWidth, showFileIcons, suffix, styles)
	name := string(panellist.RunesFromDisplay(display))
	if nameOnly {
		return fmt.Sprintf("%-*s", width, name)
	}
	if tw == 0 {
		if showMeta {
			metaPadded := padMetaLineToWidth(metaText, metaColW)
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
	if showMeta {
		metaPadded := padMetaLineToWidth(metaText, metaColW)
		return fmt.Sprintf("%-*s  %s %*s  %-*s", nameWidth, name, metaPadded, panelListSizeCells, formatListedSize(entry, painter), tw, third)
	}
	return fmt.Sprintf("%-*s %*s  %-*s", nameWidth, name, panelListSizeCells, formatListedSize(entry, painter), tw, third)
}

func matchSpans(entry localfs.Entry, rowWidth int, ranges []search.Range, highlightCursor bool, styles theme.Theme, showFileIcons bool, suffix panellist.RowSuffix, showMeta bool, metaColW int, listFmt panel.ListFormat, nameOnly, showGit bool, nameBGAt func(displayIndex int) tcell.Style) []primitive.Span {
	if len(ranges) == 0 {
		return nil
	}
	listFmt = panel.EffectiveListFormat(listFmt)
	// rowWidth already has the git strip excluded; pass false to avoid double-subtracting.
	nameWidth := panelListNameWidth(rowWidth, listFmt, nameOnly, false)
	if showMeta {
		nameWidth = panelListNameWidthWithMeta(rowWidth, metaColW, listFmt, nameOnly, false)
	}
	display := panellist.EntryDisplayRunes(entry, nameWidth, showFileIcons, suffix, styles)
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
