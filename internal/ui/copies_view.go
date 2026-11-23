package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// CopiesViewState is the Other copies view: the copies of one file as a flat list in the first pane
// and a real file-list panel (Model.CopiesPanel) in the second, on the directory of the cursor copy.
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
	// Pane holds the cursor (Selected), scroll and type-to-jump filter over Files.
	Pane DedupPane
	// FocusPanel: the embedded panel has focus instead of the list.
	FocusPanel bool
	MarkSet
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

// CopiesRects are the list's and the embedded panel's rects: the list is always the first
// (left/top) pane, whichever browser panel is active (auxiliary views ignore SwapPanes). Single
// source for drawing, visible-row counts and hit tests.
func CopiesRects(layout Layout) (list, panelRect Rect) {
	return layout.Primary, layout.Secondary
}

// CopiesListRows is the number of rows the copies list in rect shows; it has no header row.
func CopiesListRows(rect Rect) int { return SelectedStripListRows(rect) }

// CopiesListTop is the screen row of the first copy in the list drawn in rect.
func CopiesListTop(rect Rect) int { return rect.Y + 1 }

// CopiesListEntryAt returns the copy row of the Other copies view drawn at (x, y).
func CopiesListEntryAt(layout Layout, model *Model, x, y int) (int, bool) {
	list, _ := CopiesRects(layout)
	view := &model.CopiesView
	return listRowAt(list, CopiesListTop(list), CopiesListRows(list), view.Pane.ListScroll, len(view.Files), x, y)
}

// CopiesPanelEntryAt returns the visible entry of the Other copies view's panel drawn at (x, y).
func CopiesPanelEntryAt(layout Layout, model *Model, x, y int) (int, bool) {
	_, rect := CopiesRects(layout)
	return auxListRowAt(rect, model.CopiesPanel.ScrollOffset, model.CopiesPanel.VisibleEntryCount(), x, y)
}

func copiesViewTitle(view *CopiesViewState) string {
	return fmt.Sprintf(" Other copies of %s (%d) ", view.Origin, len(view.Files))
}

// drawCopiesView paints the copies list and the embedded panel.
func drawCopiesView(screen tcell.Screen, layout Layout, model *Model, styles theme.Theme, chromeBlocked bool) {
	view := &model.CopiesView
	listRect, panelRect := CopiesRects(layout)
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
	contentX, listW := rect.X+2, rect.Width-4
	visibleRows := CopiesListRows(rect)
	if visibleRows <= 0 {
		return
	}
	y := CopiesListTop(rect)
	if len(view.Files) == 0 {
		primitive.Text(screen, contentX, y, listW, "No copies left", styles.JobsRow.Background(bg))
		return
	}
	scroll := auxListScroll(view.Pane.ListScroll, len(view.Files), visibleRows)
	base := styles.PanelRowFile.Background(bg)
	for row := range visibleRows {
		idx := scroll + row
		if idx >= len(view.Files) {
			break
		}
		f := view.Files[idx]
		lineY := y + row
		selected := view.Marked[f.Path]
		rowSelected := idx == view.Pane.Selected

		lineStyle, _ := treeListRowStyle(styles, base, bg, selected, rowSelected,
			!chromeBlocked && model.AuxListHover.Active && model.AuxListHover.Row == idx,
			theme.PanelListingCursorOpts{ChromeBlocked: chromeBlocked, FileListActive: focused})
		primitive.Text(screen, rect.X+1, lineY, rect.Width-2, "", lineStyle)

		name := filepath.Base(f.Path)
		text := primitive.FitPathForWidth(primitive.PathWithHomeTilde(f.Path, model.UserHomeDir), listW)
		var spans []primitive.Span
		if strings.HasSuffix(text, name) { // the filter matches names; a shortened one has no ranges to show
			off := utf8.RuneCountInString(text) - utf8.RuneCountInString(name)
			matchStyle := styles.FuzzyHighlight
			if rowSelected && focused {
				matchStyle = styles.FuzzyHighlightCursor
			}
			_, rowBG, _ := lineStyle.Decompose()
			for _, r := range view.Pane.Filter.Ranges(idx) {
				spans = append(spans, primitive.Span{Start: off + r.Start, End: off + r.End, Style: matchStyle.Background(rowBG)})
			}
		}
		primitive.StyledText(screen, contentX, lineY, listW, text, lineStyle, spans)
	}
}
