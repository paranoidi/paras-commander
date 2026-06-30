package ui

import (
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// PanelBottomIndicatorID identifies a bottom-border status segment on file panels.
type PanelBottomIndicatorID string

const (
	PanelBottomIndicatorSelections     PanelBottomIndicatorID = "selections"
	PanelBottomIndicatorDotfilesHidden PanelBottomIndicatorID = "dotfiles_hidden"
	PanelBottomIndicatorGitignore      PanelBottomIndicatorID = "gitignore"
	PanelBottomIndicatorStash          PanelBottomIndicatorID = "stash"
	PanelBottomIndicatorSync           PanelBottomIndicatorID = "sync"
	PanelBottomIndicatorQuickView      PanelBottomIndicatorID = "quick_view"
	PanelBottomIndicatorOtherPanel     PanelBottomIndicatorID = "other_panel"
)

// PanelBottomEdge names which horizontal edge of the panel bottom row an indicator uses.
type PanelBottomEdge int

const (
	// PanelBottomEdgeStart is the panel-relative start corner (physical left on PrimaryPanel,
	// physical right on SecondaryPanel). Used for cross-directory Selections.
	PanelBottomEdgeStart PanelBottomEdge = iota
	// PanelBottomEdgePhysicalLeft chains segments from the physical left interior column
	// (dotfiles-hidden glyph, Gitignore, stash, and trailing frame dashes on both panels).
	PanelBottomEdgePhysicalLeft
	// PanelBottomEdgeEnd is the panel-relative end corner (sync, quick view, hidden other path).
	PanelBottomEdgeEnd
)

// PanelBottomIndicatorContext carries panel chrome inputs for visibility and styling.
type PanelBottomIndicatorContext struct {
	PanelID                int
	State                  panel.State
	SelectionsBottomHint   bool
	SyncDriverPanelID      int
	QuickViewDriverPanelID int
	HideInactivePanel      bool
	ActivePanel            int
	OtherPanelPath         string
	UserHomeDir            string
	EndEdgePathMaxRunes    int
	FileListActive         bool
	ChromeBlocked          bool
	BorderStyle            tcell.Style
	Styles                 theme.Theme
	// SelectionSizeLabel is the padded bottom-row text (leading/trailing space included).
	SelectionSizeLabel string
	// SelectionSizeWidth is the rune width of SelectionSizeLabel (0 when unset).
	SelectionSizeWidth int
	// SelectionSizeCenterStart/End are inclusive screen columns for the centered label.
	SelectionSizeCenterStart int
	SelectionSizeCenterEnd   int
	SplitOrientation         SplitOrientation
}

type panelBottomIndicatorSpec struct {
	ID    PanelBottomIndicatorID
	Edge  PanelBottomEdge
	Order int
}

// panelBottomIndicatorRegistry is the single source of truth for segment order.
var panelBottomIndicatorRegistry = []panelBottomIndicatorSpec{
	{ID: PanelBottomIndicatorSelections, Edge: PanelBottomEdgeStart, Order: 0},
	{ID: PanelBottomIndicatorDotfilesHidden, Edge: PanelBottomEdgePhysicalLeft, Order: 0},
	{ID: PanelBottomIndicatorGitignore, Edge: PanelBottomEdgePhysicalLeft, Order: 1},
	{ID: PanelBottomIndicatorStash, Edge: PanelBottomEdgePhysicalLeft, Order: 2},
	{ID: PanelBottomIndicatorSync, Edge: PanelBottomEdgeEnd, Order: 0},
	{ID: PanelBottomIndicatorQuickView, Edge: PanelBottomEdgeEnd, Order: 0},
	{ID: PanelBottomIndicatorOtherPanel, Edge: PanelBottomEdgeEnd, Order: 1},
}

type panelBottomIndicatorSegment struct {
	ID    PanelBottomIndicatorID
	Edge  PanelBottomEdge
	Order int
	Label string
	Style tcell.Style
}

// panelBottomIndicatorStyle resolves segment paint style via theme panel.indicator.* (with
// documented fallbacks). Dotfiles-hidden and gitignore still default to panel frame when unset.
func panelBottomIndicatorStyle(ctx PanelBottomIndicatorContext, id PanelBottomIndicatorID) tcell.Style {
	switch id {
	case PanelBottomIndicatorGitignore, PanelBottomIndicatorDotfilesHidden:
		return ctx.BorderStyle
	default:
		return ctx.Styles.PanelBottomIndicator(string(id), ctx.FileListActive, ctx.ChromeBlocked)
	}
}

// panelBottomIndicatorVisible reports whether an indicator should appear for the context.
func panelBottomIndicatorVisible(id PanelBottomIndicatorID, ctx PanelBottomIndicatorContext) bool {
	switch id {
	case PanelBottomIndicatorSelections:
		return ctx.SelectionsBottomHint
	case PanelBottomIndicatorDotfilesHidden:
		return !ctx.State.ShowHidden && ctx.State.DotfilesHiddenActive
	case PanelBottomIndicatorGitignore:
		return ctx.State.GitignoreActive
	case PanelBottomIndicatorStash:
		return ctx.State.StashPathCount() > 0
	case PanelBottomIndicatorSync:
		return ctx.SyncDriverPanelID == ctx.PanelID
	case PanelBottomIndicatorQuickView:
		return ctx.QuickViewDriverPanelID == ctx.PanelID
	case PanelBottomIndicatorOtherPanel:
		return ctx.HideInactivePanel && ctx.PanelID == ctx.ActivePanel && ctx.OtherPanelPath != ""
	default:
		return false
	}
}

func panelBottomIndicatorLabel(id PanelBottomIndicatorID, ctx PanelBottomIndicatorContext) string {
	switch id {
	case PanelBottomIndicatorSelections:
		return panelSelectionsChromePadded
	case PanelBottomIndicatorDotfilesHidden:
		return panelDotfilesHiddenChromePadded(ctx.Styles)
	case PanelBottomIndicatorGitignore:
		return panelGitignoreChromePadded
	case PanelBottomIndicatorStash:
		n := ctx.State.StashPathCount()
		if n == 0 {
			return ""
		}
		sym := ctx.Styles.SymbolStash()
		word := "selection"
		if n != 1 {
			word = "selections"
		}
		return fmt.Sprintf(" %s %d %s stashed ", sym, n, word)
	case PanelBottomIndicatorSync:
		return panelSyncIndicatorLabel(ctx.PanelID, ctx.SplitOrientation)
	case PanelBottomIndicatorQuickView:
		return panelQuickViewIndicatorLabel(ctx.PanelID, ctx.SplitOrientation)
	case PanelBottomIndicatorOtherPanel:
		return panelOtherPanelIndicatorLabel(ctx.PanelID, ctx)
	default:
		return ""
	}
}

func panelDotfilesHiddenChromePadded(styles theme.Theme) string {
	return " " + styles.SymbolHiddenDotfiles() + " "
}

// panelBottomPhysicalLeftChainStartX is the first column for the physical-left chain after any
// selections-hint offset (preserves legacy layout when cross-dir selections use the corner).
func panelBottomPhysicalLeftChainStartX(rect Rect, selectionsBottomHint bool) int {
	x := rect.X + 1
	if selectionsBottomHint {
		selPadW := utf8.RuneCountInString(panelSelectionsChromePadded)
		x += 1 + selPadW
	}
	return x
}

func panelBottomEndEdgeSegments(ctx PanelBottomIndicatorContext) []panelBottomIndicatorSegment {
	var end []panelBottomIndicatorSegment
	for _, spec := range panelBottomIndicatorRegistry {
		if spec.Edge != PanelBottomEdgeEnd {
			continue
		}
		if !panelBottomIndicatorVisible(spec.ID, ctx) {
			continue
		}
		label := panelBottomIndicatorLabel(spec.ID, ctx)
		if label == "" {
			continue
		}
		end = append(end, panelBottomIndicatorSegment{
			ID:    spec.ID,
			Edge:  spec.Edge,
			Order: spec.Order,
			Label: label,
			Style: panelBottomIndicatorStyle(ctx, spec.ID),
		})
	}
	sort.SliceStable(end, func(i, j int) bool {
		return end[i].Order < end[j].Order
	})
	return end
}

// panelBottomEndEdgeTotalWidth returns rune width reserved on the End edge for all visible segments.
func panelBottomEndEdgeTotalWidth(ctx PanelBottomIndicatorContext) int {
	total := 0
	for _, seg := range panelBottomEndEdgeSegments(ctx) {
		total += utf8.RuneCountInString(seg.Label)
	}
	return total
}

// panelBottomEdgeAvailableWidth is interior bottom-row width minus center selection-size reserve.
func panelBottomEdgeAvailableWidth(rect Rect, ctx PanelBottomIndicatorContext) int {
	w := rect.Width - 2
	if ctx.SelectionSizeWidth > 0 {
		w -= ctx.SelectionSizeWidth
	}
	return max(0, w)
}

// panelBottomEndEdgeReservedStart returns the first column (inclusive) still available on the
// bottom interior row before End-edge indicator overlays on that row.
func panelBottomEndEdgeReservedStart(rect Rect, ctx PanelBottomIndicatorContext) int {
	lastIn := rect.X + rect.Width - 2
	if !panelEndEdgeOnBottomRow(ctx.PanelID, ctx.SplitOrientation) {
		return lastIn
	}
	labelW := panelBottomEndEdgeTotalWidth(ctx)
	if labelW == 0 || labelW > rect.Width-2 {
		return lastIn
	}
	var endReserved int
	if ctx.PanelID == SecondaryPanel {
		endReserved = rect.X + labelW
	} else {
		endReserved = lastIn - labelW
	}
	if ctx.SelectionSizeCenterStart > 0 {
		if ctx.PanelID == SecondaryPanel {
			if endReserved > ctx.SelectionSizeCenterStart-1 {
				endReserved = ctx.SelectionSizeCenterStart - 1
			}
		} else if endReserved > ctx.SelectionSizeCenterStart-1 {
			endReserved = ctx.SelectionSizeCenterStart - 1
		}
	}
	return endReserved
}

// finalizeBottomCtx computes the derived fields of ctx that depend on rect:
// SelectionSizeLabel centering and EndEdgePathMaxRunes path budget.
func finalizeBottomCtx(rect Rect, ctx *PanelBottomIndicatorContext) {
	if ctx.SelectionSizeLabel != "" {
		padded, startX, endX, ok := panelSelectionSizeCenterLayout(rect, ctx.SelectionSizeLabel)
		if ok {
			ctx.SelectionSizeLabel = padded
			ctx.SelectionSizeWidth = utf8.RuneCountInString(padded)
			ctx.SelectionSizeCenterStart = startX
			ctx.SelectionSizeCenterEnd = endX
		}
	}
	avail := panelBottomEdgeAvailableWidth(rect, *ctx)
	fixed := 0
	for _, spec := range panelBottomIndicatorRegistry {
		if spec.Edge != PanelBottomEdgeEnd || spec.ID == PanelBottomIndicatorOtherPanel {
			continue
		}
		if !panelBottomIndicatorVisible(spec.ID, *ctx) {
			continue
		}
		fixed += utf8.RuneCountInString(panelBottomIndicatorLabel(spec.ID, *ctx))
	}
	if ctx.HideInactivePanel && ctx.PanelID == ctx.ActivePanel && ctx.OtherPanelPath != "" {
		ctx.EndEdgePathMaxRunes = max(0, avail-fixed-4)
	} else {
		ctx.EndEdgePathMaxRunes = max(0, avail-fixed)
	}
}

// collectPanelBottomIndicators returns visible segments sorted by edge then order.
func collectPanelBottomIndicators(ctx PanelBottomIndicatorContext) []panelBottomIndicatorSegment {
	var out []panelBottomIndicatorSegment
	for _, spec := range panelBottomIndicatorRegistry {
		if !panelBottomIndicatorVisible(spec.ID, ctx) {
			continue
		}
		label := panelBottomIndicatorLabel(spec.ID, ctx)
		if label == "" {
			continue
		}
		out = append(out, panelBottomIndicatorSegment{
			ID:    spec.ID,
			Edge:  spec.Edge,
			Order: spec.Order,
			Label: label,
			Style: panelBottomIndicatorStyle(ctx, spec.ID),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Edge != out[j].Edge {
			return out[i].Edge < out[j].Edge
		}
		return out[i].Order < out[j].Order
	})
	return out
}

// dropPanelBottomIndicatorsForWidth removes lowest-priority segments on an edge until the
// listed segments fit in maxCols (each segment needs a leading dash except the first on PhysicalLeft when offset already drew one — handled by caller).
func dropPanelBottomIndicatorsForWidth(segs []panelBottomIndicatorSegment, maxCols int, leadingDash bool) []panelBottomIndicatorSegment {
	if maxCols <= 0 || len(segs) == 0 {
		return nil
	}
	for len(segs) > 0 {
		need := 0
		if leadingDash {
			need++
		}
		for i, seg := range segs {
			if i > 0 || leadingDash {
				need++
			}
			need += utf8.RuneCountInString(seg.Label)
		}
		if need <= maxCols {
			return segs
		}
		// Drop highest Order on this edge (last in stable sort for same edge).
		drop := len(segs) - 1
		for i := len(segs) - 2; i >= 0; i-- {
			if segs[i].Order >= segs[drop].Order {
				drop = i
			}
		}
		segs = append(segs[:drop], segs[drop+1:]...)
	}
	return nil
}

// drawPanelEndEdgeIndicators paints End-edge registry segments on a frame row (top or bottom).
func drawPanelEndEdgeIndicators(screen tcell.Screen, rect Rect, panelID int, ctx PanelBottomIndicatorContext, y int) {
	if ctx.ChromeBlocked {
		return
	}
	segs := panelBottomEndEdgeSegments(ctx)
	if len(segs) == 0 {
		return
	}
	available := panelBottomEdgeAvailableWidth(rect, ctx)
	totalW := panelBottomEndEdgeTotalWidth(ctx)
	if totalW > available {
		segs = dropPanelBottomIndicatorsForWidth(segs, available, false)
	}
	if len(segs) == 0 {
		return
	}
	if panelID == SecondaryPanel {
		// End edge on the inner-left; higher Order extends toward physical right.
		x := rect.X + 1
		for _, seg := range segs {
			w := utf8.RuneCountInString(seg.Label)
			if x+w-1 > rect.X+rect.Width-2 {
				return
			}
			primitive.TextOverlay(screen, x, y, w, seg.Label, seg.Style)
			x += w
		}
		return
	}
	// Primary panel: anchor at physical right; higher Order is rightmost (at the corner).
	x := rect.X + rect.Width - 1
	for i := len(segs) - 1; i >= 0; i-- {
		seg := segs[i]
		w := utf8.RuneCountInString(seg.Label)
		x -= w
		if x < rect.X+1 {
			return
		}
		primitive.TextOverlay(screen, x, y, w, seg.Label, seg.Style)
	}
}

// drawPanelBottomEndEdgeIndicators paints End-edge registry segments on the bottom frame row.
func drawPanelBottomEndEdgeIndicators(screen tcell.Screen, rect Rect, panelID int, ctx PanelBottomIndicatorContext) {
	if !panelEndEdgeOnBottomRow(panelID, ctx.SplitOrientation) {
		return
	}
	drawPanelEndEdgeIndicators(screen, rect, panelID, ctx, rect.Y+rect.Height-1)
}

// drawPanelTopEndEdgeIndicators paints End-edge registry segments on the top frame row (stacked secondary).
func drawPanelTopEndEdgeIndicators(screen tcell.Screen, rect Rect, panelID int, ctx PanelBottomIndicatorContext) {
	if !panelEndEdgeOnTopRow(panelID, ctx.SplitOrientation) {
		return
	}
	drawPanelEndEdgeIndicators(screen, rect, panelID, ctx, rect.Y)
}

// drawPanelBottomIndicators paints Start-edge, PhysicalLeft-edge, and End-edge registry segments.
func drawPanelBottomIndicators(screen tcell.Screen, rect Rect, ctx PanelBottomIndicatorContext) {
	if rect.Width <= 4 || rect.Height < 2 {
		return
	}
	all := collectPanelBottomIndicators(ctx)
	y := rect.Y + rect.Height - 1
	lastIn := rect.X + rect.Width - 2
	endX := panelBottomEndEdgeReservedStart(rect, ctx)

	var startEdge, physicalLeft []panelBottomIndicatorSegment
	for _, seg := range all {
		switch seg.Edge {
		case PanelBottomEdgeStart:
			startEdge = append(startEdge, seg)
		case PanelBottomEdgePhysicalLeft:
			physicalLeft = append(physicalLeft, seg)
		}
	}

	drawPanelBottomStartEdgeIndicators(screen, rect, ctx, y, lastIn, startEdge)

	if len(physicalLeft) > 0 {
		x := panelBottomPhysicalLeftChainStartX(rect, ctx.SelectionsBottomHint)
		if x <= endX {
			leadingDash := !ctx.SelectionsBottomHint
			maxCols := endX - x + 1
			if ctx.SelectionSizeCenterStart > 0 {
				maxCols = min(maxCols, ctx.SelectionSizeCenterStart-x)
			}
			physicalLeft = dropPanelBottomIndicatorsForWidth(physicalLeft, maxCols, leadingDash)
			if len(physicalLeft) > 0 {
				if ctx.SelectionsBottomHint {
					screen.SetContent(x, y, '─', nil, ctx.BorderStyle)
					x++
				} else {
					screen.SetContent(x, y, '─', nil, ctx.BorderStyle)
					x++
				}
				for i, seg := range physicalLeft {
					if i > 0 {
						if x > endX {
							break
						}
						screen.SetContent(x, y, '─', nil, ctx.BorderStyle)
						x++
					}
					padW := utf8.RuneCountInString(seg.Label)
					if x+padW-1 > endX {
						break
					}
					primitive.TextOverlay(screen, x, y, padW, seg.Label, seg.Style)
					x += padW
				}
				for xi := x; xi <= endX; xi++ {
					screen.SetContent(xi, y, '─', nil, ctx.BorderStyle)
				}
			}
		}
	}

	drawPanelBottomEndEdgeIndicators(screen, rect, ctx.PanelID, ctx)
	drawPanelTopEndEdgeIndicators(screen, rect, ctx.PanelID, ctx)
}

// drawPanelBottomStartEdgeIndicators paints corner-anchored segments (Selections).
func drawPanelBottomStartEdgeIndicators(screen tcell.Screen, rect Rect, ctx PanelBottomIndicatorContext, y, lastIn int, segs []panelBottomIndicatorSegment) {
	if len(segs) == 0 {
		return
	}
	available := panelBottomEdgeAvailableWidth(rect, ctx)
	segs = dropPanelBottomIndicatorsForWidth(segs, available, true)
	for _, seg := range segs {
		padW := utf8.RuneCountInString(seg.Label)
		need := 1 + padW
		if need > available {
			continue
		}
		if ctx.PanelID == SecondaryPanel {
			xTitle := lastIn - padW
			primitive.TextOverlay(screen, xTitle, y, padW, seg.Label, seg.Style)
			screen.SetContent(lastIn, y, '─', nil, ctx.BorderStyle)
			continue
		}
		x0 := rect.X + 1
		screen.SetContent(x0, y, '─', nil, ctx.BorderStyle)
		primitive.TextOverlay(screen, x0+1, y, padW, seg.Label, seg.Style)
	}
}
