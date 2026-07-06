package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

const dedupSizeCol = 12 // " 1.2M ×3"

func drawDedupView(
	screen tcell.Screen,
	layout Layout,
	view DedupViewState,
	snap comparepkg.DedupSnapshot,
	list []DedupEntry,
	styles theme.Theme,
	chromeBlocked bool,
	userHomeDir string,
	orientation SplitOrientation,
) {
	rect := MergeTwinPanelRects(layout.Primary, layout.Secondary, orientation)
	title := dedupViewTitle(snap, view.IgnoredEmptyCount)
	// Sort order / marked indicator only applies to the finished results list.
	endLabel := ""
	if snap.Phase == comparepkg.DedupDone {
		endLabel = panelSelectionSizePadded(dedupEndLabel(view))
	}
	layoutChrome := drawAuxPanelChrome(screen, rect, title, endLabel, true, chromeBlocked, styles)
	bg := layoutChrome.ContentBG

	contentX := rect.X + 2
	contentW := max(rect.Width-4, 1)

	headerY := rect.Y + 1
	y := headerY + 1
	headerStyle := layoutChrome.Chrome.Header
	rootHeader := primitive.FitPathForWidth(primitive.PathWithHomeTilde(snap.Root.String(), userHomeDir), contentW)
	primitive.Text(screen, rect.X+1, headerY, rect.Width-2, "", headerStyle)
	primitive.Text(screen, contentX, headerY, contentW, rootHeader, headerStyle)

	visibleRows := PanelListRows(rect)
	if visibleRows <= 0 {
		return
	}

	// Full-screen prompts take over the body.
	if snap.Phase == comparepkg.DedupAwaitConfirm {
		drawDedupCenter(screen, rect, y, visibleRows,
			fmt.Sprintf("%d files to hash — Enter to continue · Esc to cancel", snap.HashTotal),
			styles.JobsRow.Background(bg))
		return
	}

	if snap.Phase == comparepkg.DedupHashing {
		drawDedupHashProgress(screen, rect.X+1, y, max(rect.Width-2, 1), snap, styles, bg)
		return
	}

	if len(list) == 0 {
		primitive.Text(screen, contentX, y, contentW, dedupEmptyMessage(snap), styles.JobsRow.Background(bg))
		return
	}

	n := len(list)
	scroll := max(view.ListScroll, 0)
	if scroll+visibleRows > n {
		scroll = max(0, n-visibleRows)
	}

	pathW := max(contentW-1-dedupSizeCol, 4)
	pathX := contentX
	gapBeforeSizeX := pathX + pathW
	sizeX := gapBeforeSizeX + 1
	innerRight := rect.X + rect.Width - 2
	sizeW := innerRight - sizeX + 1

	base := styles.JobsRow.Background(bg)
	dim := styles.PanelBlockedText.Background(bg)
	activeStart, activeEnd := dedupGroupBounds(list, view.Selected)
	for row := range visibleRows {
		idx := scroll + row
		if idx >= n {
			break
		}
		entry := list[idx]
		lineY := y + row
		marked := view.Marked[entry.AbsKey]
		rowSelected := idx == view.Selected
		groupAllMarked := DedupGroupFullyMarked(list, view.Marked, idx)

		rowBase := base
		if idx < activeStart || idx >= activeEnd {
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
				FileListActive: true,
				Selected:       marked,
			})
		case marked:
			lineStyle = styles.PanelListingSelectedStyle(chromeBlocked).Background(bg)
		}

		primitive.Text(screen, rect.X+1, lineY, 1, "", lineStyle)

		pathText := primitive.FitPathForWidth(entry.File.Rel, pathW)
		if !entry.GroupFirst {
			pathText = primitive.FitPathForWidth("↳ "+entry.File.Rel, pathW)
		}
		primitive.Text(screen, pathX, lineY, pathW, pathText, lineStyle)
		primitive.Text(screen, gapBeforeSizeX, lineY, 1, "", lineStyle)

		if entry.GroupFirst {
			sizeText := fmt.Sprintf("%s ×%d", formatJobBytes(entry.Size), entry.Copies)
			sizeStyle := dim
			if rowSelected || groupAllMarked {
				sizeStyle = lineStyle
			}
			primitive.Text(screen, sizeX, lineY, sizeW, sizeText, sizeStyle)
		} else {
			primitive.Text(screen, sizeX, lineY, sizeW, "", lineStyle)
		}
	}
}

// drawDedupHashProgress paints a disk-usage-style fill meter across the row, with the
// currently-hashing path as the overlaid label.
func drawDedupHashProgress(screen tcell.Screen, x, y, width int, snap comparepkg.DedupSnapshot, styles theme.Theme, bg tcell.Color) {
	if width <= 0 {
		return
	}
	pct := 0.0
	if snap.HashTotal > 0 {
		pct = float64(snap.Hashed) / float64(snap.HashTotal)
	}
	pct = min(1, max(0, pct))
	fill := int(pct * float64(width))

	label := "Hashing files…"
	if snap.Current != "" {
		label = "Hashing " + snap.Current + "…"
	}
	labelRunes := []rune(truncateMiddle(label, width))

	base := styles.JobsRow.Background(bg)
	accent := mergeDiskUsageBackground(base, styles.DiskUsageBarStyle(true, false, false))
	for i := range width {
		r := " "
		if i < len(labelRunes) {
			r = string(labelRunes[i])
		}
		st := base
		if i < fill {
			st = accent
		}
		primitive.Text(screen, x+i, y, 1, r, st)
	}
}

// drawDedupCenter paints a single centered line in the list body.
func drawDedupCenter(screen tcell.Screen, rect Rect, bodyY, bodyRows int, text string, style tcell.Style) {
	innerLeft := rect.X + 1
	innerRight := rect.X + rect.Width - 2
	innerW := innerRight - innerLeft + 1
	if innerW <= 0 || bodyRows <= 0 {
		return
	}
	text = primitive.FitPathForWidth(text, innerW)
	runes := len([]rune(text))
	x := innerLeft + (innerW-runes)/2
	yy := bodyY + bodyRows/2
	primitive.TextOverlay(screen, x, yy, runes, text, style)
}

// dedupEndLabel is the top-right border indicator: current sort order, plus the
// marked-files summary when any rows are marked. Mirrors the compare view's
// filter indicator (endLabel via FilterLabel).
func dedupEndLabel(view DedupViewState) string {
	sortLabel := "Sort: Path"
	if view.SortByWasted {
		sortLabel = "Sort: Wasted"
	}
	if ms := view.MarkedSummary(); ms != "" {
		return sortLabel + " · " + ms
	}
	return sortLabel
}

func dedupViewTitle(snap comparepkg.DedupSnapshot, ignoredEmpty int) string {
	switch snap.Phase {
	case comparepkg.DedupWalking:
		return " Duplicates (walking…) "
	case comparepkg.DedupAwaitConfirm:
		return " Duplicates (confirm) "
	case comparepkg.DedupHashing:
		if snap.HashTotal > 0 {
			return fmt.Sprintf(" Duplicates (hash %d/%d) ", snap.Hashed, snap.HashTotal)
		}
		return " Duplicates (hashing…) "
	case comparepkg.DedupError:
		return " Duplicates (error) "
	case comparepkg.DedupCanceled:
		return " Duplicates (canceled) "
	default:
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
}

func dedupEmptyMessage(snap comparepkg.DedupSnapshot) string {
	switch snap.Phase {
	case comparepkg.DedupWalking:
		return " Walking directory… "
	case comparepkg.DedupError:
		if snap.Err != "" {
			return " " + snap.Err + " "
		}
		return " Find duplicates failed "
	default:
		return " No duplicate files found "
	}
}
