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
	PanelBottomIndicatorJobWrite       PanelBottomIndicatorID = "job_write"
	PanelBottomIndicatorEntryFilter    PanelBottomIndicatorID = "entry_filter"
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
	// (dotfiles-hidden icon, Gitignore, stash, and trailing frame dashes on both panels).
	PanelBottomEdgePhysicalLeft
	// PanelBottomEdgePhysicalRight chains segments from the physical right interior column
	// on the bottom row of both panels (job_write). Unlike End, this is never the inner-left
	// edge of Secondary and never moves to the top row in a vertical split.
	PanelBottomEdgePhysicalRight
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
	// JobWriteMark / JobWriteStatus report whether this panel's current directory is
	// inside a non-finished job's write (destination) tree; see PanelInsideJobWriteTree.
	JobWriteMark   bool
	JobWriteStatus string
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
	{ID: PanelBottomIndicatorEntryFilter, Edge: PanelBottomEdgePhysicalLeft, Order: 4},
	{ID: PanelBottomIndicatorJobWrite, Edge: PanelBottomEdgePhysicalRight, Order: 0},
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

// panelBottomIndicatorStyle resolves segment paint style via theme panel.status.* (with
// documented fallbacks).
func panelBottomIndicatorStyle(ctx PanelBottomIndicatorContext, id PanelBottomIndicatorID) tcell.Style {
	switch id {
	case PanelBottomIndicatorJobWrite:
		return ctx.Styles.PanelJobMarkStyle(ctx.JobWriteStatus, true)
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
	case PanelBottomIndicatorJobWrite:
		return ctx.JobWriteMark
	case PanelBottomIndicatorEntryFilter:
		return ctx.State.ActiveEntryFilter != nil
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
		sym := ctx.Styles.IconStash()
		word := "selection"
		if n != 1 {
			word = "selections"
		}
		return fmt.Sprintf(" %s %d %s stashed ", sym, n, word)
	case PanelBottomIndicatorJobWrite:
		return " " + string(ctx.Styles.IconFilelistJob()) + " "
	case PanelBottomIndicatorEntryFilter:
		if f := ctx.State.ActiveEntryFilter; f != nil {
			return " " + f.Label + " "
		}
		return ""
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
	return " " + styles.IconHiddenDotfiles() + " "
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
	return panelBottomEdgeSegments(ctx, PanelBottomEdgeEnd)
}

func panelBottomPhysicalRightSegments(ctx PanelBottomIndicatorContext) []panelBottomIndicatorSegment {
	return panelBottomEdgeSegments(ctx, PanelBottomEdgePhysicalRight)
}

func panelBottomEdgeSegments(ctx PanelBottomIndicatorContext, edge PanelBottomEdge) []panelBottomIndicatorSegment {
	var out []panelBottomIndicatorSegment
	for _, spec := range panelBottomIndicatorRegistry {
		if spec.Edge != edge {
			continue
		}
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
		return out[i].Order < out[j].Order
	})
	return out
}

// panelBottomEndEdgeTotalWidth returns rune width reserved on the End edge for all visible segments.
func panelBottomEndEdgeTotalWidth(ctx PanelBottomIndicatorContext) int {
	return panelBottomSegmentsTotalWidth(panelBottomEndEdgeSegments(ctx))
}

// panelBottomPhysicalRightTotalWidth returns rune width of all visible PhysicalRight segments.
func panelBottomPhysicalRightTotalWidth(ctx PanelBottomIndicatorContext) int {
	return panelBottomSegmentsTotalWidth(panelBottomPhysicalRightSegments(ctx))
}

func panelBottomSegmentsTotalWidth(segs []panelBottomIndicatorSegment) int {
	total := 0
	for _, seg := range segs {
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

// panelBottomPhysicalRightLastFree returns the last interior column that left/center content may
// use before PhysicalRight overlays (always on the bottom row for both panels).
func panelBottomPhysicalRightLastFree(rect Rect, ctx PanelBottomIndicatorContext) int {
	lastIn := rect.X + rect.Width - 2
	labelW := panelBottomPhysicalRightTotalWidth(ctx)
	if labelW == 0 || labelW > rect.Width-2 {
		return lastIn
	}
	return lastIn - labelW
}

// panelBottomEndEdgeReservedStart returns the last column physical-left / center content may use
// on the bottom interior row before End-edge (and PhysicalRight) overlays. For Secondary when End
// paints on the left, the return is the last column of those left End overlays (callers that need
// the right-side free limit also consult panelBottomPhysicalRightLastFree).
func panelBottomEndEdgeReservedStart(rect Rect, ctx PanelBottomIndicatorContext) int {
	rightFree := panelBottomPhysicalRightLastFree(rect, ctx)
	if !panelEndEdgeOnBottomRow(ctx.PanelID, ctx.SplitOrientation) {
		return rightFree
	}
	labelW := panelBottomEndEdgeTotalWidth(ctx)
	if labelW == 0 || labelW > rect.Width-2 {
		return rightFree
	}
	var endReserved int
	if ctx.PanelID == SecondaryPanel {
		endReserved = rect.X + labelW
	} else {
		// Primary End sits immediately left of PhysicalRight.
		endReserved = rightFree - labelW
	}
	if ctx.SelectionSizeCenterStart > 0 && endReserved > ctx.SelectionSizeCenterStart-1 {
		endReserved = ctx.SelectionSizeCenterStart - 1
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
	fixed := panelBottomPhysicalRightTotalWidth(*ctx)
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
	// Primary panel: anchor left of PhysicalRight (or at physical right when none); higher Order
	// is rightmost within the End block.
	prW := 0
	if y == rect.Y+rect.Height-1 {
		prW = panelBottomPhysicalRightTotalWidth(ctx)
	}
	x := rect.X + rect.Width - 1 - prW
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

// drawPanelPhysicalRightIndicators paints PhysicalRight registry segments on the bottom frame row
// of both panels (physical right interior, independent of End-edge secondary-top behavior).
func drawPanelPhysicalRightIndicators(screen tcell.Screen, rect Rect, ctx PanelBottomIndicatorContext) {
	if ctx.ChromeBlocked {
		return
	}
	segs := panelBottomPhysicalRightSegments(ctx)
	if len(segs) == 0 {
		return
	}
	available := panelBottomEdgeAvailableWidth(rect, ctx)
	totalW := panelBottomPhysicalRightTotalWidth(ctx)
	if totalW > available {
		segs = dropPanelBottomIndicatorsForWidth(segs, available, false)
	}
	if len(segs) == 0 {
		return
	}
	y := rect.Y + rect.Height - 1
	// Anchor at physical right border; higher Order is rightmost (at the corner).
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

// drawPanelBottomIndicators paints Start-edge, PhysicalLeft-edge, End-edge, and PhysicalRight segments.
func drawPanelBottomIndicators(screen tcell.Screen, rect Rect, ctx PanelBottomIndicatorContext) {
	if rect.Width <= 4 || rect.Height < 2 {
		return
	}
	all := collectPanelBottomIndicators(ctx)
	y := rect.Y + rect.Height - 1
	lastIn := rect.X + rect.Width - 2
	// Physical-left dash fill stops before End (Primary) / PhysicalRight (both).
	leftMax := panelBottomEndEdgeReservedStart(rect, ctx)
	if ctx.PanelID == SecondaryPanel {
		leftMax = panelBottomPhysicalRightLastFree(rect, ctx)
	}

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
		if x <= leftMax {
			leadingDash := !ctx.SelectionsBottomHint
			maxCols := leftMax - x + 1
			if ctx.SelectionSizeCenterStart > 0 {
				maxCols = min(maxCols, ctx.SelectionSizeCenterStart-x)
			}
			physicalLeft = dropPanelBottomIndicatorsForWidth(physicalLeft, maxCols, leadingDash)
			if len(physicalLeft) > 0 {
				screen.SetContent(x, y, '─', nil, ctx.BorderStyle)
				x++
				for i, seg := range physicalLeft {
					if i > 0 {
						if x > leftMax {
							break
						}
						screen.SetContent(x, y, '─', nil, ctx.BorderStyle)
						x++
					}
					padW := utf8.RuneCountInString(seg.Label)
					if x+padW-1 > leftMax {
						break
					}
					primitive.TextOverlay(screen, x, y, padW, seg.Label, seg.Style)
					x += padW
				}
				for xi := x; xi <= leftMax; xi++ {
					screen.SetContent(xi, y, '─', nil, ctx.BorderStyle)
				}
			}
		}
	}

	drawPanelBottomEndEdgeIndicators(screen, rect, ctx.PanelID, ctx)
	drawPanelTopEndEdgeIndicators(screen, rect, ctx.PanelID, ctx)
	drawPanelPhysicalRightIndicators(screen, rect, ctx)
}

// drawPanelBottomStartEdgeIndicators paints corner-anchored segments (Selections).
func drawPanelBottomStartEdgeIndicators(screen tcell.Screen, rect Rect, ctx PanelBottomIndicatorContext, y, lastIn int, segs []panelBottomIndicatorSegment) {
	if len(segs) == 0 {
		return
	}
	available := panelBottomEdgeAvailableWidth(rect, ctx)
	segs = dropPanelBottomIndicatorsForWidth(segs, available, true)
	prW := panelBottomPhysicalRightTotalWidth(ctx)
	for _, seg := range segs {
		padW := utf8.RuneCountInString(seg.Label)
		need := 1 + padW
		if need > available {
			continue
		}
		if ctx.PanelID == SecondaryPanel {
			// Physical right outer corner holds job_write; Selections sits just left of it.
			xTitle := lastIn - padW - prW
			if xTitle < rect.X+1 {
				continue
			}
			primitive.TextOverlay(screen, xTitle, y, padW, seg.Label, seg.Style)
			if prW == 0 {
				screen.SetContent(lastIn, y, '─', nil, ctx.BorderStyle)
			}
			continue
		}
		x0 := rect.X + 1
		screen.SetContent(x0, y, '─', nil, ctx.BorderStyle)
		primitive.TextOverlay(screen, x0+1, y, padW, seg.Label, seg.Style)
	}
}
