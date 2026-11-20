package ui

import (
	"path/filepath"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/selstrip"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

const (
	// panelSelectedChromeName is the cross-directory selections title word (strip + inactive file-panel bottom hint).
	panelSelectedChromeName = "Selected"
	// panelSelectedChromePadded is the strip title / bottom-hint segment with one space on each side of the name.
	panelSelectedChromePadded = " " + panelSelectedChromeName + " "
)

// SelectedStripOpts carries display configuration for drawSelectedStrip.
type SelectedStripOpts struct {
	Styles                          theme.Theme
	UserHomeDir                     string
	Painter                         DiskUsagePainter
	DiskUsageDescendIntoMountPoints bool
	DiskUsageGoduIgnore             func(string) bool
	ShowSelectionSizeOnBottom       bool
	ScrollbarStyle                  uiscrollbar.Style
	ScrollbarShowInactive           bool
	PanelFileListActive             bool
	ViMotionActive                  bool
}

// SelectedStripData is everything drawSelectedStrip paints, independent of the owner
// (file panel or find-duplicates view). Paths are absolute and already ordered.
type SelectedStripData struct {
	Paths     []string
	Strip     selstrip.State
	BaseDir   string // display paths are relative to this directory
	EndLabel  string // top-border end label (multi-location icon); hidden while the filter is shown
	SizeLabel string // bottom-border selection count/size; empty = none
}

// drawBrowserSelectedStrip draws a file panel's selected strip.
func drawBrowserSelectedStrip(
	screen tcell.Screen,
	rect Rect,
	state panel.State,
	stripFocused, chromeBlocked bool,
	opts SelectedStripOpts,
) {
	data := SelectedStripData{
		Paths:   state.SelectedStripPaths(),
		Strip:   state.Strip,
		BaseDir: state.Path.String(),
	}
	if _, multiDir, _ := state.SelectedCommonRoot(); multiDir {
		data.EndLabel = " " + opts.Styles.IconSelectedMultiLocation() + " "
	}
	if opts.ShowSelectionSizeOnBottom {
		data.SizeLabel, _ = SelectionSizeLabel(
			&state,
			!state.Path.HasSubtreeSizes(),
			opts.Painter,
			opts.Styles.IconWorking(),
		)
	}
	drawSelectedStrip(screen, rect, data, stripFocused, chromeBlocked, opts)
}

// drawSelectedStrip renders a list of selected paths (the file panel's selections outside the
// current directory, or the find-duplicates marks).
// The title is always "Selected"; stripFocused only affects title active vs inactive color.
func drawSelectedStrip(
	screen tcell.Screen,
	rect Rect,
	data SelectedStripData,
	stripFocused, chromeBlocked bool,
	opts SelectedStripOpts,
) {
	styles := opts.Styles
	userHomeDir := opts.UserHomeDir
	scrollbarStyle := opts.ScrollbarStyle
	scrollbarShowInactive := opts.ScrollbarShowInactive
	panelFileListActive := opts.PanelFileListActive
	if rect.Height <= 0 || rect.Width < 8 {
		return
	}

	endLabel := data.EndLabel
	filterUI := stripFocused && data.Strip.Filter.UIActive()
	title := panelSelectedChromePadded
	if filterUI {
		title = ""
		endLabel = "" // filter query owns the top row
	}
	chrome := drawAuxPanelChrome(screen, rect, title, endLabel, stripFocused, chromeBlocked, opts.ViMotionActive, styles)
	if filterUI {
		drawQuickFilterTitle(screen, chrome.TitleX, rect.Y, chrome.TitleWidth, data.Strip.Filter, styles)
	}

	if data.SizeLabel != "" {
		endStyle := styles.PanelBottomIndicator(theme.PanelBottomIndicatorKeySelectionSize, stripFocused, chromeBlocked)
		paintSelectedStripBottomSize(screen, rect, data.SizeLabel, endStyle, chrome.Chrome.Frame)
	}

	visibleRows := SelectedStripListRows(rect)
	if visibleRows == 0 {
		return
	}

	paths := data.Paths
	interior := rect.Width - 2
	rowTextWidth := max(1, interior)
	contentStart := rect.X + 1

	scroll := data.Strip.Scroll
	if scroll < 0 {
		scroll = 0
	}

	markSource := styles.PanelRowSelected
	if chromeBlocked {
		markSource = styles.PanelBlockedRowSelected
	}

	mark := styles.IconFilelistSelectionSubtree()
	selectedStripMarkPrefix := " " + string(mark) + " "
	markCols := utf8.RuneCountInString(selectedStripMarkPrefix)
	markStart := 1
	markEnd := markStart + utf8.RuneCountInString(string(mark))

	for row := 0; row < visibleRows; row++ {
		y := rect.Y + 1 + row
		idx := scroll + row
		baseStyle := styles.PanelRowFile
		cursorStyleKey := ""
		if chromeBlocked {
			baseStyle = styles.PanelBlockedRowFile
		}
		if stripFocused && idx == data.Strip.Cursor {
			if chromeBlocked {
				baseStyle = styles.PanelBlockedCursor
				cursorStyleKey = "panel.blocked.row.cursor"
			} else {
				baseStyle = styles.PanelCursorActive
				cursorStyleKey = "panel.active.row.cursor"
			}
		}

		prefix := ""
		textCols := rowTextWidth
		if idx < len(paths) {
			prefix = selectedStripMarkPrefix
			textCols = rowTextWidth - markCols
			if textCols < 0 {
				textCols = 0
			}
		}

		text := ""
		pathLabel := ""
		if idx < len(paths) {
			p := paths[idx]
			pathLabel = selectionStripDisplayPath(p, data.BaseDir, userHomeDir, textCols)
			text = prefix + pathLabel
		}

		var spans []primitive.Span
		if prefix != "" {
			spans = []primitive.Span{{
				Start: markStart,
				End:   markEnd,
				Style: baseStyle.Foreground(styles.PanelRowIconForeground(cursorStyleKey, markSource)),
			}}
		}
		if idx < len(paths) {
			baseRanges := data.Strip.Filter.Ranges(idx)
			if mapped := selstrip.MapBasenameRangesToDisplay(pathLabel, paths[idx], baseRanges); len(mapped) > 0 {
				matchStyle := styles.FuzzyHighlight
				if stripFocused && idx == data.Strip.Cursor {
					matchStyle = styles.FuzzyHighlightCursor
				}
				_, bg, _ := baseStyle.Decompose()
				ms := matchStyle.Background(bg)
				for _, r := range mapped {
					spans = append(spans, primitive.Span{
						Start: markCols + r.Start,
						End:   markCols + r.End,
						Style: ms,
					})
				}
			}
		}
		primitive.StyledText(screen, contentStart, y, rowTextWidth, text, baseStyle, spans)
	}

	drawPanelListScrollbar(screen, rect,
		panelScrollPos{ListTopY: rect.Y + 1, Visible: visibleRows, Total: len(paths), Offset: scroll},
		scrollbarStyle, panelScrollbarShow(panelFileListActive || stripFocused, scrollbarShowInactive),
		stripFocused, chromeBlocked, chrome.Chrome.Frame, styles)
}

// paintSelectedStripBottomSize paints the padded selection count/size on the bottom border,
// right-aligned with a frame dash immediately before the corner (… label ─┘).
func paintSelectedStripBottomSize(screen tcell.Screen, rect Rect, rawLabel string, endStyle, borderStyle tcell.Style) {
	padded := panelSelectionSizePadded(rawLabel)
	w := utf8.RuneCountInString(padded)
	if w == 0 {
		return
	}
	y := rect.Y + rect.Height - 1
	firstIn := rect.X + 1
	innerRight := rect.X + rect.Width - 2
	if innerRight < firstIn {
		return
	}
	labelEndX := innerRight - 1
	if labelEndX < firstIn {
		return
	}
	avail := labelEndX - firstIn + 1
	if w > avail {
		padded = primitive.TruncateRight(padded, avail)
		w = utf8.RuneCountInString(padded)
		if w == 0 {
			return
		}
	}
	endStartX := labelEndX - w + 1
	primitive.TextOverlay(screen, endStartX, y, w, padded, endStyle)
	screen.SetContent(innerRight, y, '─', nil, borderStyle)
}

// selectionStripDisplayPath shows the path relative to the current directory
// (name.txt, child/name.txt, ../sibling.txt); tilde-absolute when Rel fails (e.g. remote vs local).
func selectionStripDisplayPath(absPath, curDir, userHomeDir string, width int) string {
	if width <= 0 {
		return ""
	}
	label, err := filepath.Rel(curDir, absPath)
	if err != nil {
		label = primitive.PathWithHomeTilde(absPath, userHomeDir)
	}
	return primitive.FitPathForWidth(label, width)
}
