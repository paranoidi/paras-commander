package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Legend texts for the cursor row's icons, shown on the tree's bottom border.
const (
	uniqueLegendUnverified    = "copies not ruled out, not fully hashed or a potential duplicate"
	uniqueLegendDirUnverified = "contains files with copies not ruled out"
	uniqueLegendMarkedSubtree = "contains marked files"
)

func uniqueViewTitle(view *UniqueViewState, styles theme.Theme) string {
	title := " Unique files"
	if view.Loaded {
		title += " (" + strconv.Itoa(len(view.VisibleFiles()))
		if n := view.UnverifiedCount(); n > 0 {
			title += fmt.Sprintf(", %d not fully hashed", n)
		}
		title += ")"
	}
	if view.Loading {
		title += fmt.Sprintf(" %c", styles.IconMenuJob("running"))
	}
	return title + " "
}

// uniqueEndLabel is the top-right border indicator: the marked-files summary.
func uniqueEndLabel(view *UniqueViewState) string {
	n := len(view.Marked)
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d marked (%s)", n, formatByteSizeListed(view.MarkedBytes))
}

// uniqueScopeHeader names what was checked, for the tree's header line.
func uniqueScopeHeader(view *UniqueViewState, homeDir string) string {
	parts := make([]string, len(view.Scope))
	for i, dir := range view.Scope {
		parts[i] = primitive.PathWithHomeTilde(dir, homeDir)
	}
	return strings.Join(parts, ", ")
}

// drawUniqueView paints the results tree and the destination panel.
func drawUniqueView(screen tcell.Screen, layout Layout, model Model, styles theme.Theme, chromeBlocked bool) {
	view := &model.UniqueView
	treeRect, destRect := UniqueRects(layout, model.ActivePanel)
	drawUniqueTree(screen, treeRect, view, model.UniqueList, uniqueTreeParams{
		Focused: !view.FocusPanel, ChromeBlocked: chromeBlocked, Header: uniqueScopeHeader(view, model.UserHomeDir),
		Empty:    uniqueEmptyText(view, model.UserHomeDir),
		RowMarks: withRangeAnchor(rowMarksResolver(model.PinnedItems, model.JobPathMarks), view.RangeAnchor),
		Hover:    model.AuxListHover,
	}, styles)
	if destRect.Width <= 0 || destRect.Height <= 0 {
		return
	}
	destID := model.inactivePanelID()
	dest := model.Secondary
	if destID == PrimaryPanel {
		dest = model.Primary
	}
	rowHover, rowHoverEntry := model.PanelRowHover.at(destID)
	drawPanel(screen, destRect, dest,
		PanelStyleConfig{Styles: styles, ScrollbarStyle: model.PanelScrollbar},
		PanelContext{
			PanelID: destID, FileListActive: view.FocusPanel, CursorRowActive: view.FocusPanel,
			ChromeBlocked: chromeBlocked, ActivePanel: model.ActivePanel,
			RowHover: rowHover, RowHoverEntry: rowHoverEntry,
			SyncDriverPanelID: -1, QuickViewDriverPanelID: -1, SplitOrientation: model.SplitOrientation,
		},
		PanelDisplayConfig{
			ShowIcons: model.UseNerdfontIcons, UserHomeDir: model.UserHomeDir, VolumeInfo: model.PanelVolumeInfo,
			Painter: model.DiskUsage, DiskUsageDescendIntoMountPoints: model.DiskUsageDescendIntoMountPoints,
			DiskUsageGoduIgnore: model.DiskUsageGoduIgnore, JobMarks: model.JobPathMarks,
			NarrowPanelsNameOnly: model.NarrowPanelsNameOnly, ScrollbarShowInactive: model.PanelScrollbarInactive,
		})
}

func uniqueEmptyText(view *UniqueViewState, homeDir string) string {
	if !view.Loaded {
		return "Reading the index…"
	}
	return "No unique files: every file in " + uniqueScopeHeader(view, homeDir) + " has a copy outside"
}

// uniqueTreeParams bundles the tree's rendering inputs.
type uniqueTreeParams struct {
	Focused       bool
	ChromeBlocked bool
	Header        string
	Empty         string
	RowMarks      dialog.RowMarksFunc
	Hover         ListRowHover
}

func uniqueRowDetailTexts(d UniqueRowData) (size, count string) {
	return formatByteSizeListed(d.Size), strconv.Itoa(d.Count)
}

func drawUniqueTree(screen tcell.Screen, rect Rect, view *UniqueViewState, rows []UniqueRow, p uniqueTreeParams, styles theme.Theme) {
	filterUI := p.Focused && view.Pane.Filter.UIActive()
	title, endLabel := uniqueViewTitle(view, styles), panelSelectionSizePadded(uniqueEndLabel(view))
	if filterUI {
		title, endLabel = "", "" // drawQuickFilterTitle owns the top row
	}
	chrome := drawAuxPanelChrome(screen, rect, title, endLabel, p.Focused, p.ChromeBlocked, false, styles)
	if filterUI {
		drawQuickFilterTitle(screen, chrome.TitleX, rect.Y, chrome.TitleWidth, view.Pane.Filter, styles)
	}
	bg := chrome.ContentBG
	contentX, innerRight := rect.X+2, rect.X+rect.Width-2

	sizeW, countW := len([]rune(dedupListSizeTitle)), len([]rune(dedupListCountTitle))
	for _, row := range rows {
		s, c := uniqueRowDetailTexts(row.Value)
		sizeW, countW = max(sizeW, len([]rune(s))), max(countW, len([]rune(c)))
	}
	cols := dedupListColumnLayout(contentX, innerRight, sizeW, countW)
	headerY := rect.Y + 1
	headerStyle := chrome.Chrome.Header
	primitive.Text(screen, rect.X+1, headerY, rect.Width-2, "", headerStyle)
	primitive.Text(screen, contentX, headerY, cols.listW, dedupListHeader(cols.pathW, cols.sizeColW, cols.countColW, primitive.FitPathForWidth(p.Header, cols.pathW)), headerStyle)
	primitive.Text(screen, innerRight, headerY, 1, "", headerStyle)

	visibleRows := PanelListRows(rect)
	if visibleRows <= 0 {
		return
	}
	y := headerY + 1
	if len(rows) == 0 {
		primitive.Text(screen, contentX, y, cols.listW, p.Empty, styles.JobsRow.Background(bg))
		primitive.Text(screen, innerRight, y, 1, "", styles.JobsRow.Background(bg))
		return
	}
	scroll := max(view.Pane.ListScroll, 0)
	if scroll+visibleRows > len(rows) {
		scroll = max(0, len(rows)-visibleRows)
	}
	base := styles.PanelRowFile.Background(bg)
	dim := styles.PanelBlockedText.Background(bg)
	markedUnder := view.markedUnderDirs()
	legend := ""
	for row := range visibleRows {
		idx := scroll + row
		if idx >= len(rows) {
			break
		}
		entry := rows[idx]
		d := entry.Value
		lineY := y + row
		marked := !d.IsDir && view.Marked[d.Path]
		fullyMarked := d.IsDir && markedUnder[d.Path] == d.Count
		rowSelected := idx == view.Pane.Selected

		lineStyle := base
		cursorStyleKey := ""
		switch {
		case rowSelected:
			opts := theme.PanelListingCursorOpts{
				ChromeBlocked: p.ChromeBlocked, FileListActive: p.Focused, Selected: marked || fullyMarked,
				RangePending: p.Focused && view.RangeAnchor != "",
			}
			lineStyle = styles.PanelListingCursorStyle(base, opts)
			cursorStyleKey = styles.PanelListingCursorIconKey(opts)
		case marked || fullyMarked:
			lineStyle = styles.PanelListingSelectedStyle(p.ChromeBlocked).Background(bg)
		case !p.ChromeBlocked && p.Hover.Active && p.Hover.Row == idx:
			lineStyle = styles.JobsRowHover
		}
		primitive.Text(screen, rect.X+1, lineY, 1, "", lineStyle)
		drawUniquePathColumn(screen, styles, p, view, entry, markedUnder[d.Path], cols, lineY, lineStyle, cursorStyleKey, idx, rowSelected && p.Focused)
		primitive.Text(screen, cols.gapBeforeCountX, lineY, 1, "", lineStyle)

		sizeText, countText := uniqueRowDetailTexts(d)
		detailStyle := dim
		if rowSelected || marked || fullyMarked {
			detailStyle = lineStyle
		}
		primitive.Text(screen, cols.countColX, lineY, cols.countColW, fmt.Sprintf("%*s", cols.countColW, countText), detailStyle)
		primitive.Text(screen, cols.countColX+cols.countColW, lineY, 1, "", lineStyle)
		primitive.Text(screen, cols.sizeColX, lineY, cols.sizeColW, fmt.Sprintf("%*s", cols.sizeColW, sizeText), detailStyle)
		primitive.Text(screen, innerRight, lineY, 1, "", lineStyle)
		if rowSelected && p.Focused {
			legend = uniqueRowLegend(styles, d, markedUnder[d.Path], p.RowMarks)
		}
	}
	drawAuxPanelBottomCenterLabel(screen, rect, legend, styles.PanelBottomIndicator(theme.PanelBottomIndicatorKeySelectionSize, true, false))
}

// drawUniquePathColumn paints the tree connector, folder icon, fitted name, the trailing pin / job
// marks, the marked-subtree mark and the unverified icon of one row.
func drawUniquePathColumn(screen tcell.Screen, styles theme.Theme, p uniqueTreeParams, view *UniqueViewState, entry UniqueRow, markedBelow int, cols dedupListColumns, lineY int, lineStyle tcell.Style, cursorStyleKey string, idx int, matchCursor bool) {
	d := entry.Value
	connector := dedupConnectorPrefix(styles, entry.Depth, entry.LastChild, entry.AncestorHasNext)
	gutter, gutterStyle := "", lineStyle
	if d.IsDir {
		gutter, gutterStyle = treeRowGutter(styles, true, true, entry.Expanded, lineStyle, p.ChromeBlocked)
	}
	prefixW := len([]rune(connector))
	if gutter != "" {
		prefixW += len([]rune(gutter)) + 1
	}
	marks := dialog.RowMarks{}
	if p.RowMarks != nil {
		marks = p.RowMarks(d.Path)
	}
	marksW := dlg.RowMarksWidth(marks)
	subtreeMark := d.IsDir && markedBelow > 0
	unverifiedIcon := styles.IconFileIndexState(localfs.IndexStatusUnhashed)
	showUnverified := d.Unverified > 0 && unverifiedIcon != 0
	fitW := cols.pathW - prefixW - marksW
	if subtreeMark {
		fitW -= 2
	}
	if showUnverified {
		fitW -= 2
	}
	text := primitive.FitPathForWidth(d.Display, max(fitW, 4))
	_, rowBG, _ := lineStyle.Decompose()
	var spans []primitive.Span
	if text == d.Display {
		matchStyle := styles.FuzzyHighlight
		if matchCursor {
			matchStyle = styles.FuzzyHighlightCursor
		}
		off := 0
		if gutter != "" {
			off = 1 // leading space before the text
		}
		for _, r := range view.Pane.Filter.Ranges(idx) {
			spans = append(spans, primitive.Span{Start: off + r.Start, End: off + r.End, Style: matchStyle.Background(rowBG)})
		}
	}
	x, right := cols.pathX, cols.pathX+cols.pathW
	primitive.Text(screen, x, lineY, cols.pathW, connector, styles.PanelRowTreeConnector.Background(rowBG))
	x += len([]rune(connector))
	if gutter != "" {
		primitive.Text(screen, x, lineY, right-x, gutter, gutterStyle)
		x += len([]rune(gutter))
		primitive.StyledText(screen, x, lineY, right-x, " "+text, lineStyle, spans)
		x++
	} else {
		primitive.StyledText(screen, x, lineY, right-x, text, lineStyle, spans)
	}
	x += runewidth.StringWidth(text)
	if marksW > 0 && x+marksW <= right {
		x += dlg.DrawRowMarksSuffix(screen, x, lineY, marksW, marks, rowBG, styles)
	}
	iconStyle := func(base tcell.Style) tcell.Style {
		return base.Background(rowBG).Foreground(styles.PanelRowIconForeground(cursorStyleKey, base))
	}
	if subtreeMark && x+1 < right {
		base := styles.PanelRowMarkSelectionSubtree
		if p.ChromeBlocked {
			base = styles.PanelBlockedRowSelected
		}
		primitive.Text(screen, x+1, lineY, 1, string(styles.IconFilelistSelectionSubtree()), lineStyle.Foreground(styles.PanelRowIconForeground(cursorStyleKey, base)))
		x += 2
	}
	if showUnverified && x+1 < right {
		primitive.Text(screen, x+1, lineY, 1, string(unverifiedIcon), iconStyle(styles.PanelFileIndexStyle(localfs.IndexStatusUnhashed)))
	}
}

// uniqueRowLegend lists "<icon> <meaning>" for the icons on the cursor row, in paint order.
func uniqueRowLegend(styles theme.Theme, d UniqueRowData, markedBelow int, rowMarks dialog.RowMarksFunc) string {
	var parts []string
	add := func(icon rune, text string) { parts = append(parts, string(icon)+" "+text) }
	if rowMarks != nil {
		m := rowMarks(d.Path)
		if m.HasJob {
			add(styles.IconFilelistJob(), dedupLegendJob)
		}
		if m.Pinned {
			add(styles.IconPinRune(), dedupLegendPinned)
		}
		if m.RangeAnchor {
			add(styles.IconFilelistRangeAnchor(), dedupLegendRangeAnchor)
		}
	}
	if d.IsDir && markedBelow > 0 {
		add(styles.IconFilelistSelectionSubtree(), uniqueLegendMarkedSubtree)
	}
	if icon := styles.IconFileIndexState(localfs.IndexStatusUnhashed); icon != 0 && d.Unverified > 0 {
		if d.IsDir {
			add(icon, uniqueLegendDirUnverified)
		} else {
			add(icon, uniqueLegendUnverified)
		}
	}
	return strings.Join(parts, ", ")
}
