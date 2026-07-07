package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

const (
	dedupListSizeTitle  = "Size"
	dedupListCountTitle = "Count"
)

// dedupListColumns holds horizontal layout for the dedup tree list header and rows.
type dedupListColumns struct {
	pathW, pathX, gapBeforeCountX, countColX, countColW, sizeColX, sizeColW, sizeColRight, innerRight, listW int
}

func dedupRowDetailTexts(d DedupRowData) (size, count string) {
	switch {
	case d.ShowSize:
		return formatByteSizeListed(d.Size), strconv.Itoa(d.Copies)
	case d.Kind == DedupRowDir:
		return formatByteSizeListed(d.WastedBytes), strconv.Itoa(d.DupCount)
	default:
		return "", ""
	}
}

func dedupListColumnWidths(rows []DedupRow) (sizeW, countW int) {
	sizeW = len([]rune(dedupListSizeTitle))
	countW = len([]rune(dedupListCountTitle))
	for _, row := range rows {
		s, c := dedupRowDetailTexts(row.Value)
		sizeW = max(sizeW, len([]rune(s)))
		countW = max(countW, len([]rune(c)))
	}
	return max(sizeW, 1), max(countW, 1)
}

// dedupListColumnLayout places Count and Size in compact trailing columns, right-aligned
// with one space between them and one inner margin cell at innerRight.
func dedupListColumnLayout(contentX, innerRight, sizeW, countW int) dedupListColumns {
	sizeColRight := innerRight - 1
	sizeColX := sizeColRight - sizeW + 1
	countColRight := sizeColX - 2
	countColX := countColRight - countW + 1
	gapBeforeCountX := countColX - 1
	pathX := contentX
	pathW := max(gapBeforeCountX-pathX, 1)
	listW := sizeColRight - contentX + 1
	return dedupListColumns{
		pathW:           pathW,
		pathX:           pathX,
		gapBeforeCountX: gapBeforeCountX,
		countColX:       countColX,
		countColW:       countW,
		sizeColX:        sizeColX,
		sizeColW:        sizeW,
		sizeColRight:    sizeColRight,
		innerRight:      innerRight,
		listW:           listW,
	}
}

// dedupListHeader formats the list header row: path label, Count, and Size columns.
func dedupListHeader(pathW, sizeW, countW int, pathLabel string) string {
	pathTitle := truncateHeaderRunes(pathW, pathLabel)
	sizeTitle := truncateHeaderRunes(sizeW, dedupListSizeTitle)
	countTitle := truncateHeaderRunes(countW, dedupListCountTitle)
	return fmt.Sprintf("%-*s %*s %*s", pathW, pathTitle, countW, countTitle, sizeW, sizeTitle)
}

func drawDedupView(
	screen tcell.Screen,
	layout Layout,
	view DedupViewState,
	snap comparepkg.DedupSnapshot,
	list []DedupRow,
	copies []DedupRow,
	styles theme.Theme,
	chromeBlocked bool,
	userHomeDir string,
	orientation SplitOrientation,
) {
	if snap.Phase != comparepkg.DedupDone {
		return
	}

	rootHeader := primitive.PathWithHomeTilde(snap.Root.String(), userHomeDir)
	drawDedupTreePane(screen, layout.Primary, dedupPaneParams{
		Title:      dedupViewTitle(snap, view.IgnoredEmptyCount),
		EndLabel:   panelSelectionSizePadded(dedupEndLabel(view)),
		Header:     rootHeader,
		Rows:       list,
		Pane:       view.Main,
		Focused:    !view.FocusCopies,
		EmptyText:  dedupEmptyMessage(snap),
		DimByGroup: !view.TreeDirs,
		ActiveGroup: func() int {
			if view.Main.Selected >= 0 && view.Main.Selected < len(list) {
				return list[view.Main.Selected].Value.GroupIdx
			}
			return -1
		}(),
	}, snap, view, styles, chromeBlocked)

	copiesHeader := ""
	copiesEmpty := " Select a file to see its copies "
	if sel, ok := dedupRowAt(list, view.Main.Selected); ok && sel.Value.Kind == DedupRowFile {
		copiesHeader = sel.Value.File.Rel
		copiesEmpty = " No other copies "
	}
	drawDedupTreePane(screen, layout.Secondary, dedupPaneParams{
		Title:       " Copies ",
		Header:      copiesHeader,
		Rows:        copies,
		Pane:        view.Copies,
		Focused:     view.FocusCopies,
		EmptyText:   copiesEmpty,
		ActiveGroup: -1,
	}, snap, view, styles, chromeBlocked)
}

func dedupRowAt(rows []DedupRow, idx int) (DedupRow, bool) {
	if idx < 0 || idx >= len(rows) {
		return DedupRow{}, false
	}
	return rows[idx], true
}

// dedupPaneParams bundles per-pane rendering inputs for drawDedupTreePane.
type dedupPaneParams struct {
	Title       string
	EndLabel    string
	Header      string
	Rows        []DedupRow
	Pane        DedupPane
	Focused     bool
	EmptyText   string
	DimByGroup  bool // groups mode: dim rows outside ActiveGroup
	ActiveGroup int
}

// drawDedupTreePane paints one tree pane: chrome, header line, and tree rows
// with expander gutter and the size/details column.
func drawDedupTreePane(
	screen tcell.Screen,
	rect Rect,
	p dedupPaneParams,
	snap comparepkg.DedupSnapshot,
	view DedupViewState,
	styles theme.Theme,
	chromeBlocked bool,
) {
	layoutChrome := drawAuxPanelChrome(screen, rect, p.Title, p.EndLabel, p.Focused, chromeBlocked, styles)
	bg := layoutChrome.ContentBG

	contentX := rect.X + 2
	innerRight := rect.X + rect.Width - 2

	headerY := rect.Y + 1
	y := headerY + 1
	headerStyle := layoutChrome.Chrome.Header
	sizeW, countW := dedupListColumnWidths(p.Rows)
	cols := dedupListColumnLayout(contentX, innerRight, sizeW, countW)
	pathHeader := primitive.FitPathForWidth(p.Header, cols.pathW)
	primitive.Text(screen, rect.X+1, headerY, rect.Width-2, "", headerStyle)
	primitive.Text(screen, contentX, headerY, cols.listW, dedupListHeader(cols.pathW, cols.sizeColW, cols.countColW, pathHeader), headerStyle)
	primitive.Text(screen, innerRight, headerY, 1, "", headerStyle)

	visibleRows := PanelListRows(rect)
	if visibleRows <= 0 {
		return
	}

	if len(p.Rows) == 0 {
		primitive.Text(screen, contentX, y, cols.listW, p.EmptyText, styles.JobsRow.Background(bg))
		primitive.Text(screen, innerRight, y, 1, "", styles.JobsRow.Background(bg))
		return
	}

	n := len(p.Rows)
	scroll := max(p.Pane.ListScroll, 0)
	if scroll+visibleRows > n {
		scroll = max(0, n-visibleRows)
	}

	pathW := cols.pathW
	pathX := cols.pathX
	gapBeforeCountX := cols.gapBeforeCountX
	countColX := cols.countColX
	sizeColX := cols.sizeColX

	base := styles.JobsRow.Background(bg)
	dim := styles.PanelBlockedText.Background(bg)
	for row := range visibleRows {
		idx := scroll + row
		if idx >= n {
			break
		}
		entry := p.Rows[idx]
		d := entry.Value
		lineY := y + row
		marked := d.Kind == DedupRowFile && view.Marked[d.AbsKey]
		rowSelected := idx == p.Pane.Selected
		groupAllMarked := d.Kind == DedupRowFile && d.GroupIdx >= 0 && d.GroupIdx < len(snap.Groups) &&
			DedupGroupFullyMarked(snap.Groups[d.GroupIdx], view.Marked)

		rowBase := base
		if p.DimByGroup && d.GroupIdx != p.ActiveGroup {
			rowBase = dim
		}
		lineStyle := rowBase
		switch {
		case groupAllMarked && rowSelected:
			lineStyle = styles.PanelDedupRowCursorAllMarked
		case groupAllMarked:
			lineStyle = styles.PanelDedupRowAllMarked.Background(bg)
		case rowSelected:
			lineStyle = styles.PanelListingCursorStyle(theme.PanelListingCursorOpts{
				ChromeBlocked:  chromeBlocked,
				FileListActive: p.Focused,
				Selected:       marked,
			})
		case marked:
			lineStyle = styles.PanelListingSelectedStyle(chromeBlocked).Background(bg)
		}

		primitive.Text(screen, rect.X+1, lineY, 1, "", lineStyle)

		indent := strings.Repeat("  ", entry.Depth)
		cursorStyleKey := ""
		if rowSelected {
			cursorStyleKey = styles.PanelListingCursorIconKey(theme.PanelListingCursorOpts{
				ChromeBlocked:  chromeBlocked,
				FileListActive: p.Focused,
				Selected:       marked,
			})
		}
		gutter, gutterStyle := dedupTreeGutter(styles, entry, lineStyle, chromeBlocked, cursorStyleKey)
		prefix := indent + gutter + " "
		pathText := primitive.FitPathForWidth(d.Display, max(pathW-len([]rune(prefix)), 4))
		x := pathX
		primitive.Text(screen, x, lineY, pathW, indent, lineStyle)
		x += len([]rune(indent))
		primitive.Text(screen, x, lineY, pathW-(x-pathX), gutter, gutterStyle)
		x += len([]rune(gutter))
		primitive.Text(screen, x, lineY, pathW-(x-pathX), " "+pathText, lineStyle)
		primitive.Text(screen, gapBeforeCountX, lineY, 1, "", lineStyle)

		sizeText, countText := dedupRowDetailTexts(d)
		detailStyle := dim
		if rowSelected || groupAllMarked {
			detailStyle = lineStyle
		}
		primitive.Text(screen, countColX, lineY, cols.countColW, fmt.Sprintf("%*s", cols.countColW, countText), detailStyle)
		primitive.Text(screen, countColX+cols.countColW, lineY, 1, "", lineStyle)
		primitive.Text(screen, sizeColX, lineY, cols.sizeColW, fmt.Sprintf("%*s", cols.sizeColW, sizeText), detailStyle)
		primitive.Text(screen, innerRight, lineY, 1, "", lineStyle)
	}
}

// dedupEndLabel is the top-right border indicator: current tree mode / sort
// order, plus the marked-files summary when any rows are marked. Mirrors the
// compare view's filter indicator (endLabel via FilterLabel). The sort toggle
// only applies to the groups tree, so dirs mode shows the view mode instead.
func dedupEndLabel(view DedupViewState) string {
	modeLabel := "Sort: Path"
	if view.SortByWasted {
		modeLabel = "Sort: Wasted"
	}
	if view.TreeDirs {
		modeLabel = "View: Dirs"
	}
	if ms := view.MarkedSummary(); ms != "" {
		return modeLabel + " · " + ms
	}
	return modeLabel
}

func dedupViewTitle(snap comparepkg.DedupSnapshot, ignoredEmpty int) string {
	groups := "groups"
	if len(snap.Groups) == 1 {
		groups = "group"
	}
	title := fmt.Sprintf(" Duplicates (%d %s", len(snap.Groups), groups)
	if ignoredEmpty > 0 {
		title += fmt.Sprintf(" · %d empty hidden", ignoredEmpty)
	}
	return title + ") "
}

func dedupTreeGutter(styles theme.Theme, entry DedupRow, lineStyle tcell.Style, chromeBlocked bool, cursorStyleKey string) (string, tcell.Style) {
	if entry.HasChildren {
		if entry.Value.Kind == DedupRowDir {
			kind := theme.FolderIconDefault
			if entry.Expanded {
				kind = theme.FolderIconOpen
			}
			gutter := styles.FolderIconGlyph(kind)
			gutterStyle := lineStyle
			if !chromeBlocked {
				iconRowStyle := styles.PanelListingEntryStyle(localfs.EntryDirectory, chromeBlocked)
				gutterStyle = lineStyle.Foreground(styles.FolderIconForeground(kind, cursorStyleKey, iconRowStyle))
			}
			return gutter, gutterStyle
		}
		gutter := string(styles.SymbolTreeExpand())
		if entry.Expanded {
			gutter = string(styles.SymbolTreeCollapse())
		}
		return gutter, lineStyle
	}
	return string(styles.SymbolTreeLeaf()), lineStyle
}

func dedupTreePrefix(styles theme.Theme, row DedupRow) string {
	indent := strings.Repeat("  ", row.Depth)
	gutter, _ := dedupTreeGutter(styles, row, tcell.StyleDefault, true, "")
	return indent + gutter + " "
}

func dedupEmptyMessage(snap comparepkg.DedupSnapshot) string {
	return " No duplicate files found "
}
