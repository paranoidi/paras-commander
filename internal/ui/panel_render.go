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
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
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

func panelListNameWidth(rowTextWidth int, f panel.ListFormat, nameOnly bool) int {
	return max(1, rowTextWidth-panelListReservedAfterName(f, nameOnly))
}

// panelListNameWidthWithMeta returns name width when the Meta column is shown.
func panelListNameWidthWithMeta(rowTextWidth, metaColW int, f panel.ListFormat, nameOnly bool) int {
	return max(1, rowTextWidth-panelListReservedAfterName(f, nameOnly)-2-metaColW)
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

func drawPanel(screen tcell.Screen, rect Rect, state panel.State, fileListActive bool, chromeBlocked bool, styles theme.Theme, showIcons bool, userHomeDir string, painter DiskUsagePainter, diskUsageDescendIntoMountPoints bool, diskUsageGoduIgnore func(string) bool, showDiskUsage bool, panelID int, jobs []JobEntry, syncDriverPanelID int, metaResults map[string]string, shrunkenShowsNameOnly bool, selectionsBottomHint bool) {
	var borderStyle tcell.Style
	var titleStyle tcell.Style
	var headerStyle tcell.Style
	if chromeBlocked {
		borderStyle = styles.PanelBlockedFrame
		titleStyle = styles.PanelBlockedTitle
		headerStyle = styles.PanelBlockedHeader
	} else {
		if fileListActive {
			borderStyle = styles.PanelActiveFrame
			titleStyle = styles.PanelActiveTitle
			headerStyle = styles.PanelActiveHeader
		} else {
			borderStyle = styles.PanelInactiveFrame
			titleStyle = styles.PanelInactiveTitle
			headerStyle = styles.PanelInactiveHeader
		}
	}

	primitive.Box(screen, primitive.Rect(rect), borderStyle)
	if syncDriverPanelID == panelID && !chromeBlocked {
		drawPanelSyncIndicator(screen, rect, panelID, styles.PanelSyncIndicator)
	}
	if selectionsBottomHint {
		drawPanelSelectionsBottomHint(screen, rect, panelID, titleStyle, borderStyle)
	}
	inner := primitive.Rect{X: rect.X + 1, Y: rect.Y + 1, Width: rect.Width - 2, Height: rect.Height - 2}
	if inner.Width > 0 && inner.Height > 0 {
		var surface tcell.Style
		if chromeBlocked {
			surface = styles.PanelBlockedSurface
		} else if fileListActive {
			surface = styles.PanelActiveSurface
		} else {
			surface = styles.PanelInactiveSurface
		}
		primitive.Fill(screen, inner, ' ', surface)
	}
	titleX := rect.X + 2
	innerRight := rect.X + rect.Width - 2
	contentCols := innerRight - titleX + 1
	if contentCols < 1 {
		contentCols = 1
	}
	const gapBeforeVolumeTitle = 2
	volumeLabel := panelVolumeFreeSpaceTitle(state.VolumeSpaceOK, state.VolumeAvailBytes, state.VolumeTotalBytes)
	volRunes := utf8.RuneCountInString(volumeLabel)
	showVolume := volumeLabel != "" && volRunes > 0 &&
		contentCols >= volRunes+gapBeforeVolumeTitle+3
	pathSlotCols := contentCols
	volumeStartX := 0
	if showVolume {
		volumeStartX = innerRight - volRunes + 1
		pathSlotCols = volumeStartX - titleX - gapBeforeVolumeTitle
		if pathSlotCols < 3 {
			showVolume = false
			pathSlotCols = contentCols
		}
	}
	titleWidth := pathSlotCols

	if fileListActive && (state.Filter.Active || state.Filter.Editing) {
		inputStyle := styles.FuzzyInput
		if state.Filter.Active && !state.FilterHasMatches() {
			inputStyle = styles.FuzzyInputNomatch
		}
		primitive.Text(screen, titleX, rect.Y, titleWidth, "> "+state.Filter.Query, inputStyle)
	} else {
		pathMax := titleWidth - 2
		if pathMax < 0 {
			pathMax = 0
		}
		title := " " + PanelTitlePath(state.Path, userHomeDir, pathMax) + " "
		primitive.TextOverlay(screen, titleX, rect.Y, titleWidth, title, titleStyle)
	}
	if showVolume {
		spaceStyle := styles.PanelInactiveSpace
		if fileListActive {
			spaceStyle = styles.PanelActiveSpace
		}
		leaderRunes := utf8.RuneCountInString(panelVolumeTitleLeader)
		trailerRunes := utf8.RuneCountInString(panelVolumeTitleTrailer)
		// Decorative dashes in leader/trailer keep the border/frame colour.
		primitive.TextOverlay(screen, volumeStartX, rect.Y, leaderRunes, panelVolumeTitleLeader, borderStyle)
		contentX := volumeStartX + leaderRunes
		contentLen := volRunes - leaderRunes - trailerRunes
		contentText := string([]rune(volumeLabel)[leaderRunes : leaderRunes+contentLen])
		primitive.TextOverlay(screen, contentX, rect.Y, contentLen, contentText, spaceStyle)
		primitive.TextOverlay(screen, volumeStartX+volRunes-trailerRunes, rect.Y, trailerRunes, panelVolumeTitleTrailer, borderStyle)
	}

	visibleRows := PanelListRows(rect)
	if visibleRows == 0 {
		return
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
	rowTextWidth := interior - leftGutter - iconStrip
	contentStart := rect.X + 1 + leftGutter
	fullRowCells := leftGutter + iconStrip + rowTextWidth
	diskDenom := panelDiskUsageDenom(showDiskUsage, painter, state.Entries)

	showMeta := metaResults != nil
	metaColW := 0
	var metaFormatted map[string]string
	if showMeta {
		metaColW, metaFormatted = layoutMetaCells(metaResults)
	}
	nameOnlyDisplay := shrunkenShowsNameOnly && rowTextWidth < config.ShrunkenListingRowTextWidthThreshold
	showMetaEffective := showMeta && !nameOnlyDisplay
	header := panelListHeader(rowTextWidth, state, showIcons, showMetaEffective, metaColW, nameOnlyDisplay)
	headerY := rect.Y + 1
	if leftGutter > 0 {
		for i := 0; i < leftGutter; i++ {
			screen.SetContent(rect.X+1+i, headerY, ' ', nil, headerStyle)
		}
	}
	if showIcons {
		paintPanelIconStripBlank(screen, contentStart, headerY, headerStyle)
	}
	primitive.Text(screen, contentStart+iconStrip, headerY, rowTextWidth, header, headerStyle)

	listFmt := panel.EffectiveListFormat(state.ListFormat)
	nameWidth := panelListNameWidth(rowTextWidth, listFmt, nameOnlyDisplay)
	if showMetaEffective {
		nameWidth = panelListNameWidthWithMeta(rowTextWidth, metaColW, listFmt, nameOnlyDisplay)
	}
	markSource := styles.PanelRowSelected
	if chromeBlocked {
		markSource = styles.PanelBlockedRowSelected
	}
	listingMarkFG, _, _ := markSource.Decompose()

	for row := 0; row < visibleRows; row++ {
		y := rect.Y + 2 + row
		style := styles.PanelRowNormal
		text := ""
		entryIndex := state.ScrollOffset + row
		var spans []primitive.Span

		hasEntry := false
		var cur localfs.Entry
		selected := false
		fillCols := 0

		var subtreeMark bool
		var jobMark bool
		var jobStatus string
		var jobMarkGlyph rune

		if entry, _, ok := state.VisibleEntry(entryIndex); ok {
			hasEntry = true
			cur = entry
			style = panelEntryStyle(entry, chromeBlocked, styles)
			selected = state.IsSelected(entry)
			if selected {
				if chromeBlocked {
					style = styles.PanelBlockedRowSelected
				} else {
					style = styles.PanelRowSelected
				}
			}
			if entryIndex == state.Cursor {
				if chromeBlocked {
					style = styles.PanelBlockedCursor
					if selected {
						style = styles.PanelBlockedCursorSelected
					}
				} else if fileListActive {
					style = styles.PanelCursorActive
					if selected {
						style = styles.PanelCursorSelected
					}
				} else {
					style = styles.PanelCursorInactive
					if selected {
						style = styles.PanelCursorSelected
					}
				}
			}
			subtreeMark = entry.Type == localfs.EntryDirectory && nameWidth > 2 && state.HasSelectionInSubtree(entry.Path)
			jobMark, jobStatus = EntryPathJobMarkStatus(entry.Path, jobs)
			if jobMark {
				glyphStr := jobRowLeadingIcon(jobStatus, styles)
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
				metaText = metaFormatted[entry.Path]
			}
			text = formatEntry(entry, rowTextWidth, showIcons, jobMarkGlyph, subtreeMark, painter, showMetaEffective, metaColW, metaText, listFmt, nameOnlyDisplay)
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

		if hasEntry {
			spans = matchSpans(cur, rowTextWidth, state.MatchRanges(entryIndex), entryIndex == state.Cursor, styles, showIcons, jobMarkGlyph, subtreeMark, showMetaEffective, metaColW, listFmt, nameOnlyDisplay, func(di int) tcell.Style {
				return blendCell(di + leftGutter + iconStrip)
			})
			if jobMark || subtreeMark {
				display := entryDisplayRunes(cur, nameWidth, showIcons, jobMarkGlyph, subtreeMark)
				suf := entryListingSuffixDecorationLen(nameWidth, jobMarkGlyph, subtreeMark && cur.Type == localfs.EntryDirectory)
				decStart := len(display) - suf
				if decStart >= 0 && decStart < len(display) {
					_, rowBG, _ := style.Decompose()
					jobIconFG, _, _ := jobIconStyle(jobStatus, styles).Decompose()
					for i := decStart; i < len(display); i++ {
						r := display[i].Rune
						if r == jobMarkGlyph || r == '○' {
							fg := jobIconFG
							if r == '○' {
								fg = listingMarkFG
							}
							spanStyle := tcell.StyleDefault.Foreground(fg).Background(rowBG)
							spans = append([]primitive.Span{{Start: i, End: i + 1, Style: spanStyle}}, spans...)
						}
					}
				}
			}
		}

		if showIcons && leftGutter > 0 {
			for i := 0; i < leftGutter; i++ {
				screen.SetContent(rect.X+1+i, y, ' ', nil, blendCell(i))
			}
		}
		iconKey := ""
		var diskPending bool
		var diskExcluded bool
		if hasEntry {
			iconKey = panelCursorIconThemeKey(fileListActive, chromeBlocked, entryIndex, state.Cursor, selected)
			diskPending = showDiskUsage && painter != nil && cur.Type == localfs.EntryDirectory && painter.PendingForPanel(cur.Path, panelID)
			if painter != nil && cur.Type == localfs.EntryDirectory {
				diskExcluded = painter.DiskScanExcluded(cur.Path, diskUsageDescendIntoMountPoints, state.ListingDevice, state.ListingDeviceValid, diskUsageGoduIgnore)
			}
		}
		if showIcons {
			if hasEntry {
				iconStripStyle := blendCell(leftGutter)
				paintPanelIconStrip(screen, contentStart, y, cur, iconStripStyle, styles, iconKey, diskPending, diskExcluded, showDiskUsage)
			} else {
				paintPanelIconStripBlank(screen, contentStart, y, style)
			}
		}
		primitive.StyledTextCellwise(screen, contentStart+iconStrip, y, rowTextWidth, text, func(ci int) tcell.Style {
			return blendCell(leftGutter + iconStrip + ci)
		}, spans)
	}
}

// panelSyncIndicatorLabel returns the label rendered on the bottom border of the sync driver.
// Left driver points right (toward the right panel); right driver points left.
func panelSyncIndicatorLabel(panelID int) string {
	if panelID == RightPanel {
		return " ← Sync "
	}
	return " Sync → "
}

// drawPanelSyncIndicator overlays the latched-sync label on the bottom border of the driver panel,
// reserving the corner glyph. Left driver paints flush to the right (next to the ┘ corner);
// right driver paints flush to the left (next to the └ corner).
func drawPanelSyncIndicator(screen tcell.Screen, rect Rect, panelID int, style tcell.Style) {
	if rect.Width <= 4 || rect.Height < 2 {
		return
	}
	label := panelSyncIndicatorLabel(panelID)
	labelW := utf8.RuneCountInString(label)
	available := rect.Width - 2
	if labelW > available {
		return
	}
	y := rect.Y + rect.Height - 1
	var x int
	if panelID == RightPanel {
		x = rect.X + 1
	} else {
		x = rect.X + rect.Width - 1 - labelW
	}
	primitive.TextOverlay(screen, x, y, labelW, label, style)
}

// drawPanelSelectionsBottomHint mirrors drawPanelSyncIndicator: it overlays compact chrome on the
// bottom frame so an inactive column still signals hidden cross-directory selections.
// One frame "─" uses borderStyle; panelSelectionsChromePadded uses titleStyle (same as the strip title).
func drawPanelSelectionsBottomHint(screen tcell.Screen, rect Rect, panelID int, titleStyle, borderStyle tcell.Style) {
	if rect.Width <= 4 || rect.Height < 2 {
		return
	}
	padW := utf8.RuneCountInString(panelSelectionsChromePadded)
	// Leading (left) or trailing (right) frame dash plus padded title.
	need := 1 + padW
	available := rect.Width - 2
	if need > available {
		return
	}
	y := rect.Y + rect.Height - 1
	lastIn := rect.X + rect.Width - 2
	if panelID == RightPanel {
		xTitle := lastIn - padW
		primitive.TextOverlay(screen, xTitle, y, padW, panelSelectionsChromePadded, titleStyle)
		screen.SetContent(lastIn, y, '─', nil, borderStyle)
		return
	}
	x0 := rect.X + 1
	screen.SetContent(x0, y, '─', nil, borderStyle)
	primitive.TextOverlay(screen, x0+1, y, padW, panelSelectionsChromePadded, titleStyle)
}

// panelCursorIconThemeKey is the semantic style key for Theme.PanelFileIconFG on the cursor row; empty otherwise.
func panelCursorIconThemeKey(fileListActive, chromeBlocked bool, entryIndex, cursor int, selected bool) string {
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
		if selected {
			return "panel.row.cursor.selected"
		}
		return "panel.row.cursor.active"
	}
	if selected {
		return "panel.row.cursor.selected"
	}
	return "panel.row.cursor.inactive"
}

func panelListHeader(rowTextWidth int, state panel.State, showIcons bool, showMeta bool, metaColW int, nameOnly bool) string {
	if nameOnly {
		nameTitle, sizeTitle, thirdTitle := state.ListColumnTitles(showIcons)
		title := panelListHeaderTitleWithSortArrow(nameTitle, sizeTitle, thirdTitle)
		title = truncateHeaderRunes(rowTextWidth, title)
		return fmt.Sprintf("%-*s", rowTextWidth, title)
	}
	listFmt := panel.EffectiveListFormat(state.ListFormat)
	tw := panelListThirdColumnWidth(listFmt, false)
	nameWidth := panelListNameWidth(rowTextWidth, listFmt, false)
	if showMeta {
		nameWidth = panelListNameWidthWithMeta(rowTextWidth, metaColW, listFmt, false)
	}
	nameTitle, sizeTitle, thirdTitle := state.ListColumnTitles(showIcons)
	nameTitle = truncateHeaderRunes(nameWidth, nameTitle)
	sizeTitle = truncateHeaderRunes(panelListSizeCells, sizeTitle)
	if tw == 0 {
		if showMeta {
			metaHdr := padMetaLineToWidth("Meta", metaColW)
			return fmt.Sprintf("%-*s  %s %*s", nameWidth, nameTitle, metaHdr, panelListSizeCells, sizeTitle)
		}
		return fmt.Sprintf("%-*s %*s", nameWidth, nameTitle, panelListSizeCells, sizeTitle)
	}
	thirdTitle = truncateHeaderRunes(tw, thirdTitle)
	if showMeta {
		metaHdr := padMetaLineToWidth("Meta", metaColW)
		return fmt.Sprintf("%-*s  %s %*s  %-*s", nameWidth, nameTitle, metaHdr, panelListSizeCells, sizeTitle, tw, thirdTitle)
	}
	return fmt.Sprintf("%-*s %*s  %-*s", nameWidth, nameTitle, panelListSizeCells, sizeTitle, tw, thirdTitle)
}

func panelEntryStyle(entry localfs.Entry, chromeBlocked bool, styles theme.Theme) tcell.Style {
	if chromeBlocked {
		switch entry.Type {
		case localfs.EntryDirectory:
			return styles.PanelBlockedRowDirectory
		case localfs.EntrySymlink:
			return styles.PanelBlockedRowSymlink
		default:
			return styles.PanelBlockedRowNormal
		}
	}
	switch entry.Type {
	case localfs.EntryDirectory:
		return styles.PanelRowDirectory
	case localfs.EntrySymlink:
		return styles.PanelRowSymlink
	default:
		return styles.PanelRowNormal
	}
}

// displayRune tracks a single displayed rune and which character in the entry name
// it corresponds to. NameIdx is -1 for decoration (prefix / suffix characters).
type displayRune struct {
	Rune    rune
	NameIdx int // -1 for prefix/suffix decoration
}

func formatEntry(entry localfs.Entry, width int, showFileIcons bool, jobMarkRune rune, subtreeSelectionMark bool, painter DiskUsagePainter, showMeta bool, metaColW int, metaText string, listFmt panel.ListFormat, nameOnly bool) string {
	listFmt = panel.EffectiveListFormat(listFmt)
	tw := panelListThirdColumnWidth(listFmt, nameOnly)
	nameWidth := panelListNameWidth(width, listFmt, nameOnly)
	if showMeta {
		nameWidth = panelListNameWidthWithMeta(width, metaColW, listFmt, nameOnly)
	}
	display := entryDisplayRunes(entry, nameWidth, showFileIcons, jobMarkRune, subtreeSelectionMark)
	name := string(runesFromDisplay(display))
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

// entryListingSuffixDecorationLen returns how many trailing runes are reserved for job + subtree selection marks.
func entryListingSuffixDecorationLen(width int, jobMarkRune rune, subtreeForDir bool) int {
	n := 0
	if jobMarkRune != 0 && width > n+2 {
		n += 2
	}
	if subtreeForDir && width > n+2 {
		n += 2
	}
	return n
}

// entryDisplayRunes builds the display rune slice for an entry name,
// including prefix (space or /), symlink suffix (@) when applicable,
// optional trailing job-queue mark (icon glyph) before subtree selection,
// and a trailing " ○" for directories that contain a strictly nested selection, truncated as needed.
func entryDisplayRunes(entry localfs.Entry, width int, showFileIcons bool, jobMarkRune rune, subtreeSelectionMark bool) []displayRune {
	showJob := jobMarkRune != 0 && width > 2
	suffixUsed := 0
	if showJob {
		suffixUsed = 2
	}
	showSub := subtreeSelectionMark && entry.Type == localfs.EntryDirectory && width > suffixUsed+2
	suffixLen := entryListingSuffixDecorationLen(width, jobMarkRune, subtreeSelectionMark && entry.Type == localfs.EntryDirectory)
	innerW := width - suffixLen
	if innerW < 1 {
		innerW = 1
	}

	prefix := " "
	if entry.Type == localfs.EntryDirectory && !showFileIcons {
		prefix = "/"
	}
	entryRunes := []rune(entry.Name)

	var body []displayRune
	body = append(body, displayRune{Rune: []rune(prefix)[0], NameIdx: -1})
	for i, r := range entryRunes {
		body = append(body, displayRune{Rune: r, NameIdx: i})
	}
	if entry.Type == localfs.EntrySymlink {
		body = append(body, displayRune{Rune: '@', NameIdx: -1})
	}

	if width <= 0 {
		return nil
	}

	var core []displayRune
	if len(body) <= innerW {
		core = body
	} else if innerW <= 3 {
		core = body[:innerW]
	} else {
		prefixWidth := (innerW - 1) / 2
		suffixWidth := innerW - prefixWidth - 1
		truncated := make([]displayRune, 0, innerW)
		truncated = append(truncated, body[:prefixWidth]...)
		truncated = append(truncated, displayRune{Rune: '~', NameIdx: -1})
		truncated = append(truncated, body[len(body)-suffixWidth:]...)
		core = truncated
	}

	if suffixLen == 0 {
		return core
	}
	out := make([]displayRune, 0, len(core)+suffixLen)
	out = append(out, core...)
	if showJob {
		out = append(out, displayRune{Rune: ' ', NameIdx: -1}, displayRune{Rune: jobMarkRune, NameIdx: -1})
	}
	if showSub {
		out = append(out, displayRune{Rune: ' ', NameIdx: -1}, displayRune{Rune: '○', NameIdx: -1})
	}
	return out
}

func runesFromDisplay(display []displayRune) []rune {
	runes := make([]rune, len(display))
	for i, dr := range display {
		runes[i] = dr.Rune
	}
	return runes
}

func matchSpans(entry localfs.Entry, rowWidth int, ranges []search.Range, highlightCursor bool, styles theme.Theme, showFileIcons bool, jobMarkRune rune, subtreeSelectionMark bool, showMeta bool, metaColW int, listFmt panel.ListFormat, nameOnly bool, nameBGAt func(displayIndex int) tcell.Style) []primitive.Span {
	if len(ranges) == 0 {
		return nil
	}
	listFmt = panel.EffectiveListFormat(listFmt)
	nameWidth := panelListNameWidth(rowWidth, listFmt, nameOnly)
	if showMeta {
		nameWidth = panelListNameWidthWithMeta(rowWidth, metaColW, listFmt, nameOnly)
	}
	display := entryDisplayRunes(entry, nameWidth, showFileIcons, jobMarkRune, subtreeSelectionMark)
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
