package ui

import (
	"fmt"
	"path/filepath"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// CopiesViewState is the Other copies view: the copies of one file as a flat list in the active
// panel's place and a real file-list panel (Model.CopiesPanel) beside it, on the directory of the
// cursor copy.
type CopiesViewState struct {
	// Open: the view is shown, or hidden behind the fullscreen viewer it opened. The embedded
	// panel exists exactly while Open.
	Open bool
	// Origin is the file the copies belong to: its name for the title.
	Origin string
	// IndexPanel: the lookup started from a fileindex:// panel, so the embedded panel and
	// open-in-panel show fileindex:// paths.
	IndexPanel bool
	// Files are the copies as host paths, sorted; Size is the index's. The type is the one of
	// Find unique files (Unverified is unused), so the transfer and delete dialogs read both views
	// alike.
	Files []UniqueFile
	// nameW and sizeW are the widest name and size text of Files (SetFiles).
	nameW, sizeW int
	// Pane holds the cursor (Selected), scroll and type-to-jump filter over Files.
	Pane DedupPane
	// FocusPanel: the embedded panel has focus instead of the list.
	FocusPanel bool
	MarkSet
}

// SetFiles replaces the listed copies and measures the name and size columns they need.
func (s *CopiesViewState) SetFiles(files []UniqueFile) {
	s.Files = files
	s.nameW, s.sizeW = 0, 0
	for _, f := range files {
		s.sizeW = max(s.sizeW, utf8.RuneCountInString(formatByteSizeListed(f.Size)))
		s.nameW = max(s.nameW, utf8.RuneCountInString(filepath.Base(f.Path)))
	}
}

// MarkedFiles is the marked copies in list order.
func (s *CopiesViewState) MarkedFiles() []UniqueFile {
	var out []UniqueFile
	for _, f := range s.Files {
		if s.Marked[f.Path] {
			out = append(out, f)
		}
	}
	return out
}

// CopiesRects are the list's and the embedded panel's rects: the list takes the active panel's
// place and the panel the inactive one's, as in Find unique files. Single source for drawing,
// visible-row counts and hit tests.
func CopiesRects(layout Layout, activePanel int) (list, panelRect Rect) {
	return UniqueRects(layout, activePanel)
}

// CopiesListEntryAt returns the copy row of the Other copies view drawn at (x, y).
func CopiesListEntryAt(layout Layout, model *Model, x, y int) (int, bool) {
	list, _ := CopiesRects(layout, model.ActivePanel)
	view := &model.CopiesView
	return auxListRowAt(list, view.Pane.ListScroll, len(view.Files), x, y)
}

// CopiesPanelEntryAt returns the visible entry of the Other copies view's panel drawn at (x, y).
func CopiesPanelEntryAt(layout Layout, model *Model, x, y int) (int, bool) {
	_, rect := CopiesRects(layout, model.ActivePanel)
	return auxListRowAt(rect, model.CopiesPanel.ScrollOffset, model.CopiesPanel.VisibleEntryCount(), x, y)
}

func copiesViewTitle(view *CopiesViewState) string {
	return fmt.Sprintf(" Other copies of %s (%d) ", view.Origin, len(view.Files))
}

// copiesColumns is the horizontal layout of the list: name, directory, size.
type copiesColumns struct {
	nameX, nameW, dirX, dirW, sizeX, sizeW, listW int
}

func copiesColumnLayout(view *CopiesViewState, contentX, innerRight int) copiesColumns {
	sizeW := max(view.sizeW, utf8.RuneCountInString(dedupListSizeTitle))
	nameW := max(view.nameW, utf8.RuneCountInString(copiesNameTitle))
	sizeRight := innerRight - 1
	sizeX := sizeRight - sizeW + 1
	avail := max(sizeX-1-contentX, 2)
	nameW = min(nameW, max(avail/2, 1))
	dirX := contentX + nameW + 1
	return copiesColumns{
		nameX: contentX, nameW: nameW, dirX: dirX, dirW: max(sizeX-1-dirX, 1),
		sizeX: sizeX, sizeW: sizeW, listW: sizeRight - contentX + 1,
	}
}

const (
	copiesNameTitle = "Name"
	copiesDirTitle  = "Directory"
)

// drawCopiesView paints the copies list and the embedded panel.
func drawCopiesView(screen tcell.Screen, layout Layout, model *Model, styles theme.Theme, chromeBlocked bool) {
	view := &model.CopiesView
	listRect, panelRect := CopiesRects(layout, model.ActivePanel)
	drawCopiesList(screen, listRect, view, model, styles, chromeBlocked)
	drawEmbeddedPanel(screen, panelRect, &model.CopiesPanel, model.CopiesPanelID, view.FocusPanel, model, styles, chromeBlocked)
}

func drawCopiesList(screen tcell.Screen, rect Rect, view *CopiesViewState, model *Model, styles theme.Theme, chromeBlocked bool) {
	focused := !view.FocusPanel
	filterUI := focused && view.Pane.Filter.UIActive()
	title, endLabel := copiesViewTitle(view), ""
	if n := len(view.Marked); n > 0 {
		endLabel = panelSelectionSizePadded(fmt.Sprintf("%d marked (%s)", n, formatByteSizeListed(view.MarkedBytes)))
	}
	if filterUI {
		title, endLabel = "", "" // drawQuickFilterTitle owns the top row
	}
	chrome := drawAuxPanelChrome(screen, rect, title, endLabel, focused, chromeBlocked, false, styles)
	if filterUI {
		drawQuickFilterTitle(screen, chrome.TitleX, rect.Y, chrome.TitleWidth, view.Pane.Filter, styles)
	}
	bg := chrome.ContentBG
	contentX, innerRight := rect.X+2, rect.X+rect.Width-2
	cols := copiesColumnLayout(view, contentX, innerRight)
	headerY := rect.Y + 1
	headerStyle := chrome.Chrome.Header
	primitive.Text(screen, rect.X+1, headerY, rect.Width-2, "", headerStyle)
	primitive.Text(screen, cols.nameX, headerY, cols.listW, fmt.Sprintf("%-*s %-*s %*s",
		cols.nameW, primitive.TruncateRight(copiesNameTitle, cols.nameW),
		cols.dirW, primitive.TruncateRight(copiesDirTitle, cols.dirW),
		cols.sizeW, primitive.TruncateRight(dedupListSizeTitle, cols.sizeW)), headerStyle)

	visibleRows := PanelListRows(rect)
	if visibleRows <= 0 {
		return
	}
	y := headerY + 1
	if len(view.Files) == 0 {
		primitive.Text(screen, contentX, y, cols.listW, "No copies left", styles.JobsRow.Background(bg))
		return
	}
	scroll := max(view.Pane.ListScroll, 0)
	if scroll+visibleRows > len(view.Files) {
		scroll = max(0, len(view.Files)-visibleRows)
	}
	base := styles.PanelRowFile.Background(bg)
	dim := styles.PanelBlockedText.Background(bg)
	for row := range visibleRows {
		idx := scroll + row
		if idx >= len(view.Files) {
			break
		}
		f := view.Files[idx]
		lineY := y + row
		selected := view.Marked[f.Path]
		rowSelected := idx == view.Pane.Selected

		lineStyle, cursorRow := treeListRowStyle(styles, base, bg, selected, rowSelected,
			!chromeBlocked && model.AuxListHover.Active && model.AuxListHover.Row == idx,
			theme.PanelListingCursorOpts{ChromeBlocked: chromeBlocked, FileListActive: focused})
		primitive.Text(screen, rect.X+1, lineY, rect.Width-2, "", lineStyle)

		name := primitive.TruncateRight(filepath.Base(f.Path), cols.nameW)
		var spans []primitive.Span
		if name == filepath.Base(f.Path) {
			matchStyle := styles.FuzzyHighlight
			if rowSelected && focused {
				matchStyle = styles.FuzzyHighlightCursor
			}
			_, rowBG, _ := lineStyle.Decompose()
			for _, r := range view.Pane.Filter.Ranges(idx) {
				spans = append(spans, primitive.Span{Start: r.Start, End: r.End, Style: matchStyle.Background(rowBG)})
			}
		}
		primitive.StyledText(screen, cols.nameX, lineY, cols.nameW, name, lineStyle, spans)
		detailStyle := treeDetailStyle(styles, lineStyle, dim, cursorRow, selected)
		dir := primitive.PathWithHomeTilde(filepath.Dir(f.Path), model.UserHomeDir)
		primitive.Text(screen, cols.dirX, lineY, cols.dirW, primitive.FitPathForWidth(dir, cols.dirW), detailStyle)
		primitive.Text(screen, cols.sizeX, lineY, cols.sizeW, fmt.Sprintf("%*s", cols.sizeW, formatByteSizeListed(f.Size)), detailStyle)
	}
}
