package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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
	rowMarks dialog.RowMarksFunc,
) {
	if snap.Phase != comparepkg.DedupDone {
		return
	}

	rootHeader := primitive.PathWithHomeTilde(snap.EffectiveDisplayRoot().String(), userHomeDir)
	markedDirs := dedupMarkedDirSet(snap, view.Marked)
	dangerMarkedDirs := dedupDangerMarkedDirSet(snap, view.Marked)
	keptDirs := dedupKeptDirSet(snap, view.Kept)
	var treeFullyMarkedDirs map[string]bool
	if view.TreeDirs {
		treeFullyMarkedDirs = dedupSnapshotFullyMarkedDirSet(snap, view.Marked)
	}
	var copyFullyMarkedDirs map[string]bool
	if sel, ok := dedupRowAt(list, view.Main.Selected); ok && sel.Value.Kind == DedupRowFile {
		copyFullyMarkedDirs = dedupCopyPaneFullyMarkedDirSet(snap, sel, view.Marked)
	}
	activeGroup := -1
	if view.Main.Selected >= 0 && view.Main.Selected < len(list) {
		activeGroup = list[view.Main.Selected].Value.GroupIdx
	}
	var hintDirs map[string]bool
	if view.TreeDirs && activeGroup >= 0 && activeGroup < len(snap.Groups) {
		hintDirs = dedupGroupDirSet(snap.Groups[activeGroup])
	}
	drawDedupTreePane(screen, layout.Primary, dedupPaneParams{
		Title:            dedupViewTitle(snap, view.IgnoredEmptyCount),
		EndLabel:         panelSelectionSizePadded(dedupEndLabel(view)),
		Header:           rootHeader,
		Rows:             list,
		Pane:             view.Main,
		Focused:          !view.FocusCopies,
		EmptyText:        dedupEmptyMessage(snap),
		DimByGroup:       !view.TreeDirs,
		ActiveGroup:      activeGroup,
		HintDirs:         hintDirs,
		FullyMarkedDirs:  treeFullyMarkedDirs,
		MarkedDirs:       markedDirs,
		DangerMarkedDirs: dangerMarkedDirs,
		KeptDirs:         keptDirs,
		RowMarks:         rowMarks,
	}, snap, view, styles, chromeBlocked)

	copiesHeader := ""
	copiesEmpty := "Select a file to see its copies"
	if sel, ok := dedupRowAt(list, view.Main.Selected); ok && sel.Value.Kind == DedupRowFile {
		copiesHeader = sel.Value.File.Rel
		copiesEmpty = "No other copies"
	}
	drawDedupTreePane(screen, layout.Secondary, dedupPaneParams{
		Title:            " Copies ",
		Header:           copiesHeader,
		Rows:             copies,
		Pane:             view.Copies,
		Focused:          view.FocusCopies,
		EmptyText:        copiesEmpty,
		ActiveGroup:      -1,
		CopiesPane:       true,
		FullyMarkedDirs:  copyFullyMarkedDirs,
		MarkedDirs:       markedDirs,
		DangerMarkedDirs: dangerMarkedDirs,
		KeptDirs:         keptDirs,
		RowMarks:         rowMarks,
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
	Title            string
	EndLabel         string
	Header           string
	Rows             []DedupRow
	Pane             DedupPane
	Focused          bool
	EmptyText        string
	DimByGroup       bool // groups mode: dim rows outside ActiveGroup
	ActiveGroup      int
	HintDirs         map[string]bool     // dirs mode: DirRel keys whose subtree contains ActiveGroup (collapsed-folder hint)
	CopiesPane       bool                // copies pane: dir rows can show fully-marked copy styling
	FullyMarkedDirs  map[string]bool     // DirRel keys whose entire descendant duplicate subtree is marked
	MarkedDirs       map[string]bool     // DirRel keys of dirs whose subtree has a marked file
	DangerMarkedDirs map[string]bool     // DirRel keys of dirs whose subtree has a fully-marked group
	KeptDirs         map[string]bool     // DirRel keys of dirs whose subtree has a kept file
	RowMarks         dialog.RowMarksFunc // resolves the pin/in-progress-job trailing marks for one row's absolute path
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
	layoutChrome := drawAuxPanelChrome(screen, rect, p.Title, p.EndLabel, p.Focused, chromeBlocked, false, styles)
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
		kept := d.Kind == DedupRowFile && view.Kept[d.AbsKey]
		marked := d.Kind == DedupRowFile && view.Marked[d.AbsKey] && !kept
		rowSelected := idx == p.Pane.Selected
		groupAllMarked := d.Kind == DedupRowFile && d.GroupIdx >= 0 && d.GroupIdx < len(snap.Groups) &&
			DedupGroupFullyMarked(snap.Groups[d.GroupIdx], view.Marked)
		dirFullyMarked := d.Kind == DedupRowDir && p.FullyMarkedDirs[d.DirRel]

		flags := dedupRowFlags{
			RowSelected:    rowSelected,
			Kept:           kept,
			Marked:         marked,
			GroupAllMarked: groupAllMarked,
			DirFullyMarked: dirFullyMarked,
		}
		lineStyle := dedupRowStyle(styles, p, d, entry, flags, chromeBlocked, base, dim, bg)

		primitive.Text(screen, rect.X+1, lineY, 1, "", lineStyle)

		cursorStyleKey := ""
		if rowSelected {
			cursorStyleKey = styles.PanelListingCursorIconKey(theme.PanelListingCursorOpts{
				ChromeBlocked:  chromeBlocked,
				FileListActive: p.Focused,
				Selected:       marked || dirFullyMarked,
			})
		}
		drawDedupPathColumn(screen, styles, p, snap, d, entry, lineY, pathX, pathW, lineStyle, cursorStyleKey, chromeBlocked)
		primitive.Text(screen, gapBeforeCountX, lineY, 1, "", lineStyle)

		drawDedupDetailColumns(screen, cols, d, lineY, lineStyle, dim, rowSelected || kept || groupAllMarked || dirFullyMarked, innerRight)
	}
}

// dedupRowFlags bundles the per-row booleans dedupRowStyle picks a style from.
type dedupRowFlags struct {
	RowSelected    bool
	Kept           bool
	Marked         bool
	GroupAllMarked bool
	DirFullyMarked bool
}

// dedupRowStyle picks the row's base and line styles from precomputed per-row flags, moved out
// of drawDedupTreePane's two stacked flag→style switches. Pure function of flags.
func dedupRowStyle(styles theme.Theme, p dedupPaneParams, d DedupRowData, entry DedupRow, f dedupRowFlags, chromeBlocked bool, base, dim tcell.Style, bg tcell.Color) tcell.Style {
	rowBase := base
	switch {
	case p.ActiveGroup >= 0 && d.Kind == DedupRowFile && d.GroupIdx == p.ActiveGroup && !f.RowSelected:
		rowBase = styles.PanelHint.Background(bg)
	case p.ActiveGroup >= 0 && d.Kind == DedupRowDir && !entry.Expanded && p.HintDirs[d.DirRel]:
		rowBase = styles.PanelHint.Background(bg)
	case p.DimByGroup && d.GroupIdx != p.ActiveGroup:
		rowBase = dim
	}
	lineStyle := rowBase
	switch {
	case f.Kept && f.RowSelected:
		lineStyle = styles.PanelDedupRowCursorKeep
	case f.Kept:
		lineStyle = styles.PanelDedupRowKeep.Background(bg)
	case f.GroupAllMarked && f.RowSelected:
		lineStyle = styles.PanelDedupRowCursorAllMarked
	case f.GroupAllMarked:
		lineStyle = styles.PanelDedupRowAllMarked.Background(bg)
	case f.RowSelected:
		lineStyle = styles.PanelListingCursorStyle(rowBase, theme.PanelListingCursorOpts{
			ChromeBlocked:  chromeBlocked,
			FileListActive: p.Focused,
			Selected:       f.Marked || f.DirFullyMarked,
		})
	case f.Marked, f.DirFullyMarked:
		lineStyle = styles.PanelListingSelectedStyle(chromeBlocked).Background(bg)
	}
	return lineStyle
}

// dedupRowAbsPath returns the absolute path for one dedup row: file rows use their own
// AbsKey directly; dir rows join snap.EffectiveDisplayRoot() with DirRel, mirroring
// apphandler/dedup.Handler.selectedDirAbs's directory-path join.
func dedupRowAbsPath(snap comparepkg.DedupSnapshot, d DedupRowData) string {
	if d.Kind == DedupRowFile {
		return d.AbsKey
	}
	root := strings.TrimSuffix(snap.EffectiveDisplayRoot().String(), "/")
	return root + "/" + d.DirRel
}

// drawDedupPathColumn paints the tree connector, expand/collapse gutter, fitted path text, the
// trailing pin/in-progress-job marks, and subtree-mark suffix for one row, moved out of
// drawDedupTreePane's per-row path column block. Pin/job marks are painted before the subtree
// glyph, matching panellist's job/pin-before-subtree suffix ordering.
func drawDedupPathColumn(screen tcell.Screen, styles theme.Theme, p dedupPaneParams, snap comparepkg.DedupSnapshot, d DedupRowData, entry DedupRow, lineY, pathX, pathW int, lineStyle tcell.Style, cursorStyleKey string, chromeBlocked bool) {
	connectorPrefix := dedupTreeConnectorPrefix(styles, entry)
	gutter, gutterStyle := dedupTreeGutter(styles, entry, lineStyle, chromeBlocked)
	prefix := connectorPrefix
	if gutter != "" {
		prefix += gutter + " "
	}
	keptSubtree := d.Kind == DedupRowDir && p.KeptDirs[d.DirRel]
	subtreeMark := keptSubtree || (d.Kind == DedupRowDir && p.MarkedDirs[d.DirRel])
	marks := dialog.RowMarks{}
	if p.RowMarks != nil {
		marks = p.RowMarks(dedupRowAbsPath(snap, d))
	}
	marksW := dialog.RowMarksWidth(marks)
	fitW := pathW - len([]rune(prefix))
	if subtreeMark {
		fitW -= 2 // room for subtree mark suffix, like panellist.SuffixDecorationLen
	}
	fitW -= marksW
	pathText := primitive.FitPathForWidth(d.Display, max(fitW, 4))
	_, rowBG, _ := lineStyle.Decompose()
	connectorStyle := styles.PanelRowTreeConnector.Background(rowBG)
	x := pathX
	primitive.Text(screen, x, lineY, pathW, connectorPrefix, connectorStyle)
	x += len([]rune(connectorPrefix))
	if gutter != "" {
		primitive.Text(screen, x, lineY, pathW-(x-pathX), gutter, gutterStyle)
		x += len([]rune(gutter))
		primitive.Text(screen, x, lineY, pathW-(x-pathX), " "+pathText, lineStyle)
		x++ // leading space before pathText
	} else {
		primitive.Text(screen, x, lineY, pathW-(x-pathX), pathText, lineStyle)
	}
	cursorX := x + len([]rune(pathText))
	if marksW > 0 && cursorX+marksW <= pathX+pathW {
		used := dialog.DrawRowMarksSuffix(screen, cursorX, lineY, marksW, marks, rowBG, styles)
		cursorX += used
	}
	if markX := cursorX + 1; subtreeMark && markX < pathX+pathW {
		var base tcell.Style
		switch {
		case keptSubtree:
			base = styles.PanelDedupRowMarkKeepSubtree
		case p.DangerMarkedDirs[d.DirRel]:
			base = styles.PanelDedupRowAllMarked
		case chromeBlocked:
			base = styles.PanelBlockedRowSelected
		default:
			base = styles.PanelRowMarkSelectionSubtree
		}
		markStyle := lineStyle.Foreground(styles.PanelRowIconForeground(cursorStyleKey, base))
		primitive.Text(screen, markX, lineY, 1, string(styles.SymbolFilelistSelectionSubtree()), markStyle)
	}
}

// drawDedupDetailColumns paints the Count and Size trailing columns for one row, moved out of
// drawDedupTreePane's detail-column paint block.
func drawDedupDetailColumns(screen tcell.Screen, cols dedupListColumns, d DedupRowData, lineY int, lineStyle, dim tcell.Style, showLineStyle bool, innerRight int) {
	sizeText, countText := dedupRowDetailTexts(d)
	detailStyle := dim
	if showLineStyle {
		detailStyle = lineStyle
	}
	primitive.Text(screen, cols.countColX, lineY, cols.countColW, fmt.Sprintf("%*s", cols.countColW, countText), detailStyle)
	primitive.Text(screen, cols.countColX+cols.countColW, lineY, 1, "", lineStyle)
	primitive.Text(screen, cols.sizeColX, lineY, cols.sizeColW, fmt.Sprintf("%*s", cols.sizeColW, sizeText), detailStyle)
	primitive.Text(screen, innerRight, lineY, 1, "", lineStyle)
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

func dedupTreeGutter(
	styles theme.Theme,
	entry DedupRow,
	lineStyle tcell.Style,
	chromeBlocked bool,
) (string, tcell.Style) {
	if entry.HasChildren {
		if entry.Value.Kind == DedupRowDir {
			// Dedup tree rows use jobs.row as line text; folder glyphs always use directory
			// blue (panel.row.directory), never jobs.row grey, open-folder cyan, or cursor/selection FG.
			// dedup always wants the *closed*-kind foreground even when expanded, so it can't
			// reuse panellist.TreeExpanderGlyph's FG (which switches to open-folder color); it
			// only reuses the glyph text.
			kind := theme.FolderIconDefault
			if entry.Expanded {
				kind = theme.FolderIconOpen
			}
			gutter := styles.FolderIconGlyph(kind)
			iconRowStyle := styles.PanelListingEntryStyle(localfs.EntryDirectory, chromeBlocked)
			iconFG := styles.FolderIconForeground(theme.FolderIconDefault, "", iconRowStyle)
			return gutter, lineStyle.Foreground(iconFG)
		}
		gutter := string(styles.SymbolTreeExpand())
		if entry.Expanded {
			gutter = string(styles.SymbolTreeCollapse())
		}
		return gutter, lineStyle
	}
	return "", lineStyle
}

func dedupTreeConnectorPrefix(styles theme.Theme, entry DedupRow) string {
	return panellist.TreeConnectorPrefix(entry.Depth, entry.LastChild, entry.AncestorHasNext, styles)
}

func dedupTreePrefix(styles theme.Theme, row DedupRow) string {
	gutter, _ := dedupTreeGutter(styles, row, tcell.StyleDefault, true)
	connector := dedupTreeConnectorPrefix(styles, row)
	if gutter == "" {
		return connector
	}
	return connector + gutter + " "
}

func dedupEmptyMessage(snap comparepkg.DedupSnapshot) string {
	return "No duplicate files found"
}
