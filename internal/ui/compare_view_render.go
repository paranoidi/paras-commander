package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

const (
	compareStatusCol  = 2
	comparePathGapCol = 1 // blank column between each path column and the status glyph
)

func drawCompareView(
	screen tcell.Screen,
	layout Layout,
	view CompareViewState,
	snap comparepkg.Snapshot,
	rows []comparepkg.Row,
	primary, secondary panel.State,
	styles theme.Theme,
	chromeBlocked bool,
	userHomeDir string,
	orientation SplitOrientation,
) {
	rect := MergeTwinPanelRects(layout.Primary, layout.Secondary, orientation)
	endLabel := panelSelectionSizePadded(comparepkg.FilterLabel(view.Filter))
	title := compareViewTitle(snap)
	layoutChrome := drawAuxPanelChrome(screen, rect, title, endLabel, true, chromeBlocked, styles)
	bg := layoutChrome.ContentBG

	contentX := rect.X + 2
	contentW := rect.Width - 4
	if contentW < 1 {
		contentW = 1
	}

	headerY := rect.Y + 1
	y := headerY + 1
	pathW := (contentW - compareStatusCol - 1) / 2
	if pathW < 8 {
		pathW = 8
	}
	statusX := contentX + pathW
	rightX := statusX + compareStatusCol + 1

	headerStyle := layoutChrome.Chrome.Header
	leftPathW := pathW - comparePathGapCol
	if leftPathW < 1 {
		leftPathW = 1
	}
	primitive.Text(screen, rect.X+1, headerY, rect.Width-2, "", headerStyle)
	primaryHeader := primitive.FitPathForWidth(primitive.PathWithHomeTilde(snap.PrimaryRoot.String(), userHomeDir), leftPathW)
	secondaryHeader := primitive.FitPathForWidth(primitive.PathWithHomeTilde(snap.SecondaryRoot.String(), userHomeDir), pathW)
	primitive.Text(screen, contentX, headerY, leftPathW, primaryHeader, headerStyle)
	primitive.Text(screen, rightX, headerY, pathW, secondaryHeader, headerStyle)

	visibleRows := rect.Y + rect.Height - 2 - y
	if visibleRows <= 0 {
		drawCompareBottomLegend(screen, rect, layoutChrome, "", styles)
		return
	}

	if len(rows) == 0 {
		msg := compareEmptyMessage(snap)
		primitive.Text(screen, contentX, y, contentW, msg, styles.JobsRow.Background(bg))
		drawCompareBottomLegend(screen, rect, layoutChrome, "", styles)
		return
	}

	n := len(rows)
	scroll := view.ListScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll+visibleRows > n {
		scroll = max(0, n-visibleRows)
	}

	for row := 0; row < visibleRows; row++ {
		idx := scroll + row
		if idx >= n {
			break
		}
		entry := rows[idx]
		lineY := y + row
		rowSelected := idx == view.Selected
		lineStyle := styles.JobsRow.Background(bg)
		if rowSelected {
			if chromeBlocked {
				lineStyle = styles.PanelBlockedCursor
			} else {
				lineStyle = styles.PanelRowSelected.Background(bg)
			}
		}

		leftPath, rightPath := compareDisplayPathPair(entry.PrimaryRel, entry.SecondaryRel)

		pathBase := styles.JobsRow.Background(bg)
		absentStyle := styles.PanelBlockedText.Background(bg)
		leftStyle := comparePathCellStyle(pathBase, view.FocusColumn == CompareColumnPrimary, rowSelected, primary.IsSelectedPath(entryPrimaryAbs(snap, entry)), styles, chromeBlocked, bg)
		rightStyle := comparePathCellStyle(pathBase, view.FocusColumn == CompareColumnSecondary, rowSelected, secondary.IsSelectedPath(entrySecondaryAbs(snap, entry)), styles, chromeBlocked, bg)

		primitive.Text(screen, rect.X+1, lineY, 1, "", leftStyle)
		if leftPath == "" {
			effectiveLeft := absentStyle
			if rowSelected && view.FocusColumn == CompareColumnPrimary {
				effectiveLeft = leftStyle
			}
			primitive.Text(screen, contentX, lineY, leftPathW, "-", effectiveLeft)
		} else {
			primitive.Text(screen, contentX, lineY, leftPathW, primitive.FitPathForWidth(leftPath, leftPathW), leftStyle)
		}
		primitive.Text(screen, contentX+leftPathW, lineY, comparePathGapCol, "", lineStyle)
		glyph := compareGlyphCentered(compareRowGlyph(styles, entry))
		primitive.Text(screen, statusX, lineY, compareStatusCol, glyph, lineStyle)
		primitive.Text(screen, rightX-1, lineY, 1, "", rightStyle)
		if rightPath == "" {
			effectiveRight := absentStyle
			if rowSelected && view.FocusColumn == CompareColumnSecondary {
				effectiveRight = rightStyle
			}
			primitive.Text(screen, rightX, lineY, pathW+1, "-", effectiveRight)
		} else {
			primitive.Text(screen, rightX, lineY, pathW+1, primitive.FitPathForWidth(rightPath, pathW), rightStyle)
		}
	}

	legend := ""
	if view.Selected >= 0 && view.Selected < len(rows) {
		row := rows[view.Selected]
		glyph := compareRowGlyph(styles, row)
		legend = comparepkg.RowLegend(row.Kind, glyph)
		if row.Err != "" {
			legend = primitive.FitPathForWidth(row.Err, rect.Width-4)
		}
	}
	drawCompareBottomLegend(screen, rect, layoutChrome, panelSelectionSizePadded(legend), styles)
}

func compareDisplayPathPair(primaryRel, secondaryRel string) (string, string) {
	return filepath.ToSlash(primaryRel), filepath.ToSlash(secondaryRel)
}

func entryPrimaryAbs(snap comparepkg.Snapshot, row comparepkg.Row) string {
	if row.PrimaryRel == "" {
		return ""
	}
	loc, err := comparepkg.JoinRel(snap.PrimaryRoot, row.PrimaryRel)
	if err != nil {
		return ""
	}
	return filepath.Clean(loc.String())
}

func entrySecondaryAbs(snap comparepkg.Snapshot, row comparepkg.Row) string {
	if row.SecondaryRel == "" {
		return ""
	}
	loc, err := comparepkg.JoinRel(snap.SecondaryRoot, row.SecondaryRel)
	if err != nil {
		return ""
	}
	return filepath.Clean(loc.String())
}

func comparePathCellStyle(pathBase tcell.Style, colFocused, rowCursor, pathSelected bool, styles theme.Theme, chromeBlocked bool, bg tcell.Color) tcell.Style {
	if colFocused && rowCursor {
		return styles.PanelListingCursorStyle(theme.PanelListingCursorOpts{
			ChromeBlocked:  chromeBlocked,
			FileListActive: true,
			Selected:       pathSelected,
		})
	}
	if pathSelected {
		return styles.PanelListingSelectedStyle(chromeBlocked).Background(bg)
	}
	return pathBase
}

func drawCompareBottomLegend(screen tcell.Screen, rect Rect, chrome AuxPanelChromeLayout, legend string, styles theme.Theme) {
	if legend == "" {
		return
	}
	style := styles.PanelBottomIndicator(theme.PanelBottomIndicatorKeySelectionSize, true, false)
	drawAuxPanelBottomCenterLabel(screen, rect, legend, style)
}

func compareViewTitle(snap comparepkg.Snapshot) string {
	switch snap.Phase {
	case comparepkg.PhaseWalking:
		return " Compare (walking…) "
	case comparepkg.PhaseHashing:
		if snap.HashTotal > 0 {
			return fmt.Sprintf(" Compare (hash %d/%d) ", snap.Hashed, snap.HashTotal)
		}
		return " Compare (hashing…) "
	case comparepkg.PhaseError:
		return " Compare (error) "
	case comparepkg.PhaseCanceled:
		return " Compare (canceled) "
	default:
		return fmt.Sprintf(" Compare (%d) ", len(snap.Rows))
	}
}

func compareEmptyMessage(snap comparepkg.Snapshot) string {
	switch snap.Phase {
	case comparepkg.PhaseWalking:
		return " Walking directories… "
	case comparepkg.PhaseHashing:
		return " Hashing files… "
	case comparepkg.PhaseError:
		if snap.Err != "" {
			return " " + snap.Err + " "
		}
		return " Compare failed "
	default:
		return " No differences in this category "
	}
}

func compareGlyphCentered(glyph string) string {
	runes := []rune(glyph)
	if len(runes) == 0 {
		return ""
	}
	if len(runes) >= compareStatusCol {
		return string(runes[:compareStatusCol])
	}
	pad := compareStatusCol - len(runes)
	left := pad / 2
	return strings.Repeat(" ", left) + glyph + strings.Repeat(" ", pad-left)
}

func compareRowGlyph(styles theme.Theme, row comparepkg.Row) string {
	if !row.HashDone && row.Kind != comparepkg.KindContentDiff && row.Err == "" {
		if row.PrimaryRel != "" && row.SecondaryRel != "" && row.PrimaryRel == row.SecondaryRel {
			return styles.SymbolComparePending()
		}
		if row.Kind == comparepkg.KindPrimaryOnly || row.Kind == comparepkg.KindSecondaryOnly || row.PrimaryRel == "" || row.SecondaryRel == "" {
			return styles.SymbolComparePending()
		}
	}
	if row.Err != "" {
		return styles.SymbolCompareError()
	}
	switch row.Kind {
	case comparepkg.KindEqual:
		return styles.SymbolCompareEqual()
	case comparepkg.KindRelocated:
		return styles.SymbolCompareRelocated()
	case comparepkg.KindPrimaryOnly:
		return styles.SymbolComparePrimaryOnly()
	case comparepkg.KindSecondaryOnly:
		return styles.SymbolCompareSecondaryOnly()
	case comparepkg.KindContentDiff:
		return styles.SymbolCompareContentDiff()
	default:
		return styles.SymbolCompareError()
	}
}
