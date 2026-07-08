package ui

import (
	"path/filepath"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

const (
	// panelSelectionsChromeName is the cross-directory selections title word (strip + inactive file-panel bottom hint).
	panelSelectionsChromeName = "Selections"
	// panelSelectionsChromePadded is the strip title / bottom-hint segment with one space on each side of the name.
	panelSelectionsChromePadded = " " + panelSelectionsChromeName + " "
)

// SelectionsStripOpts carries display configuration for drawSelectionsStrip.
type SelectionsStripOpts struct {
	Styles                          theme.Theme
	UserHomeDir                     string
	Painter                         DiskUsagePainter
	DiskUsageDescendIntoMountPoints bool
	DiskUsageGoduIgnore             func(string) bool
	ShowSelectionSizeOnBottom       bool
	ScrollbarStyle                  uiscrollbar.Style
	ScrollbarShowInactive           bool
	PanelFileListActive             bool
}

// drawSelectionsStrip renders the per-panel list of selected paths outside the current directory.
// The title is always "Selections"; stripFocused only affects title active vs inactive color.
func drawSelectionsStrip(
	screen tcell.Screen,
	rect Rect,
	state panel.State,
	stripFocused, chromeBlocked bool,
	opts SelectionsStripOpts,
) {
	styles := opts.Styles
	userHomeDir := opts.UserHomeDir
	painter := opts.Painter
	diskUsageDescendIntoMountPoints := opts.DiskUsageDescendIntoMountPoints
	diskUsageGoduIgnore := opts.DiskUsageGoduIgnore
	showSelectionSizeOnBottom := opts.ShowSelectionSizeOnBottom
	scrollbarStyle := opts.ScrollbarStyle
	scrollbarShowInactive := opts.ScrollbarShowInactive
	panelFileListActive := opts.PanelFileListActive
	if rect.Height <= 0 || rect.Width < 8 {
		return
	}

	chrome := drawAuxPanelChrome(screen, rect, panelSelectionsChromePadded, "", stripFocused, chromeBlocked, styles)

	if showSelectionSizeOnBottom {
		if raw, ok := SelectionSizeLabel(
			&state,
			state.Path.IsRemote(),
			painter,
			diskUsageDescendIntoMountPoints,
			diskUsageGoduIgnore,
			styles.SymbolWorking(),
		); ok {
			endStyle := styles.PanelBottomIndicator(theme.PanelBottomIndicatorKeySelectionSize, stripFocused, chromeBlocked)
			paintSelectionsStripBottomSize(screen, rect, raw, endStyle, chrome.Chrome.Frame)
		}
	}

	visibleRows := SelectionsStripListRows(rect)
	if visibleRows == 0 {
		return
	}

	paths := state.SelectionsStripPaths()
	interior := rect.Width - 2
	rowTextWidth := max(1, interior)
	contentStart := rect.X + 1

	scroll := state.SelectionsStripScroll
	if scroll < 0 {
		scroll = 0
	}

	markSource := styles.PanelRowSelected
	if chromeBlocked {
		markSource = styles.PanelBlockedRowSelected
	}
	markFG, _, _ := markSource.Decompose()

	const selectionsStripMarkPrefix = " ○ "
	markCols := utf8.RuneCountInString(selectionsStripMarkPrefix)

	for row := 0; row < visibleRows; row++ {
		y := rect.Y + 1 + row
		idx := scroll + row
		baseStyle := styles.PanelRowFile
		if chromeBlocked {
			baseStyle = styles.PanelBlockedRowFile
		}
		if stripFocused && idx == state.SelectionsStripCursor {
			if chromeBlocked {
				baseStyle = styles.PanelBlockedCursor
			} else {
				baseStyle = styles.PanelCursorActive
			}
		}

		prefix := ""
		textCols := rowTextWidth
		if idx < len(paths) {
			prefix = selectionsStripMarkPrefix
			textCols = rowTextWidth - markCols
			if textCols < 0 {
				textCols = 0
			}
		}

		text := ""
		if idx < len(paths) {
			p := paths[idx]
			text = prefix + selectionStripDisplayPath(p, state.Path.String(), userHomeDir, textCols)
		}

		var spans []primitive.Span
		if prefix != "" {
			spans = []primitive.Span{{
				Start: 1,
				End:   2,
				Style: baseStyle.Foreground(markFG),
			}}
		}
		primitive.StyledText(screen, contentStart, y, rowTextWidth, text, baseStyle, spans)
	}

	drawPanelListScrollbar(screen, rect,
		panelScrollPos{ListTopY: rect.Y + 1, Visible: visibleRows, Total: len(paths), Offset: scroll},
		scrollbarStyle, panelScrollbarShow(panelFileListActive, scrollbarShowInactive),
		stripFocused, chromeBlocked, chrome.Chrome.Frame, styles)
}

// paintSelectionsStripBottomSize paints the padded selection count/size on the bottom border,
// right-aligned with a frame dash immediately before the corner (… label ─┘).
func paintSelectionsStripBottomSize(screen tcell.Screen, rect Rect, rawLabel string, endStyle, borderStyle tcell.Style) {
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
