package ui

import (
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

const (
	// panelSelectionsChromeName is the cross-directory selections title word (strip + inactive file-panel bottom hint).
	panelSelectionsChromeName = "Selections"
	// panelSelectionsChromePadded is the strip title / bottom-hint segment with one space on each side of the name.
	panelSelectionsChromePadded = " " + panelSelectionsChromeName + " "
)

// drawSelectionsStrip renders the per-panel list of selected paths outside the current directory.
// The title is always "Selections"; stripFocused only affects title active vs inactive color.
func drawSelectionsStrip(screen tcell.Screen, rect Rect, state panel.State, stripFocused, chromeBlocked bool, styles theme.Theme, userHomeDir string) {
	if rect.Height <= 0 || rect.Width < 8 {
		return
	}

	var borderStyle tcell.Style
	var titleStyle tcell.Style
	if chromeBlocked {
		borderStyle = styles.PanelBlockedFrame
		titleStyle = styles.PanelBlockedTitle
	} else {
		borderStyle = styles.PanelFrame
		titleStyle = styles.PanelTitleInactive
		if stripFocused {
			titleStyle = styles.PanelTitleActive
		}
	}

	primitive.Box(screen, primitive.Rect(rect), borderStyle)
	inner := primitive.Rect{X: rect.X + 1, Y: rect.Y + 1, Width: rect.Width - 2, Height: rect.Height - 2}
	if inner.Width > 0 && inner.Height > 0 {
		var surface tcell.Style
		if chromeBlocked {
			surface = styles.PanelBlockedSurface
		} else {
			surface = styles.PanelSurface
		}
		primitive.Fill(screen, inner, ' ', surface)
	}

	titleX := rect.X + 2
	titleWidth := rect.Width - 4
	primitive.TextOverlay(screen, titleX, rect.Y, titleWidth, panelSelectionsChromePadded, titleStyle)

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

	const selectionsStripDirMarkPrefix = " ○ "
	dirMarkCols := utf8.RuneCountInString(selectionsStripDirMarkPrefix)

	for row := 0; row < visibleRows; row++ {
		y := rect.Y + 1 + row
		idx := scroll + row
		baseStyle := styles.PanelRowNormal
		if chromeBlocked {
			baseStyle = styles.PanelBlockedRowNormal
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
		if idx < len(paths) && selectionStripPathIsDir(paths[idx]) {
			prefix = selectionsStripDirMarkPrefix
			textCols = rowTextWidth - dirMarkCols
			if textCols < 0 {
				textCols = 0
			}
		}

		text := ""
		if idx < len(paths) {
			p := paths[idx]
			text = prefix + selectionStripDisplayPath(p, userHomeDir, textCols)
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
}

func selectionStripPathIsDir(abs string) bool {
	info, err := os.Stat(filepath.Clean(abs))
	return err == nil && info.IsDir()
}

func selectionStripDisplayPath(absPath, userHomeDir string, width int) string {
	if width <= 0 {
		return ""
	}
	label := primitive.PathWithHomeTilde(absPath, userHomeDir)
	return primitive.FitPathForWidth(label, width)
}
