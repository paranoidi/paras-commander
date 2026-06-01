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
	// PanelBottomEdgeStart is the panel-relative start corner (physical left on LeftPanel,
	// physical right on RightPanel). Used for cross-directory Selections.
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

func panelOtherPanelIndicatorLabel(panelID int, ctx PanelBottomIndicatorContext) string {
	max := ctx.EndEdgePathMaxRunes
	if max <= 0 {
		max = 12
	}
	path := PanelTitlePath(ctx.OtherPanelPath, ctx.UserHomeDir, max)
	if panelID == RightPanel {
		return " ← " + path + " "
	}
	return " → " + path + " "
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
		return panelSyncIndicatorLabel(ctx.PanelID)
	case PanelBottomIndicatorQuickView:
		return panelQuickViewIndicatorLabel(ctx.PanelID)
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

// panelBottomEndEdgeReservedStart returns the first column (inclusive) still available on the
// bottom interior row before End-edge indicator overlays.
func panelBottomEndEdgeReservedStart(rect Rect, ctx PanelBottomIndicatorContext) int {
	lastIn := rect.X + rect.Width - 2
	labelW := panelBottomEndEdgeTotalWidth(ctx)
	if labelW == 0 || labelW > rect.Width-2 {
		return lastIn
	}
	if ctx.PanelID == RightPanel {
		return rect.X + labelW
	}
	return lastIn - labelW
}

// panelBottomIndicatorContextForRect builds indicator context with path budget for the other-panel label.
func panelBottomIndicatorContextForRect(
	rect Rect,
	panelID int,
	state panel.State,
	selectionsBottomHint bool,
	syncDriverPanelID, quickViewDriverPanelID int,
	hideInactivePanel bool,
	activePanel int,
	otherPanelPath, userHomeDir string,
	fileListActive, chromeBlocked bool,
	borderStyle tcell.Style,
	styles theme.Theme,
) PanelBottomIndicatorContext {
	ctx := PanelBottomIndicatorContext{
		PanelID:                panelID,
		State:                  state,
		SelectionsBottomHint:   selectionsBottomHint,
		SyncDriverPanelID:      syncDriverPanelID,
		QuickViewDriverPanelID: quickViewDriverPanelID,
		HideInactivePanel:      hideInactivePanel,
		ActivePanel:            activePanel,
		OtherPanelPath:         otherPanelPath,
		UserHomeDir:            userHomeDir,
		FileListActive:         fileListActive,
		ChromeBlocked:          chromeBlocked,
		BorderStyle:            borderStyle,
		Styles:                 styles,
	}
	avail := rect.Width - 2
	fixed := 0
	for _, spec := range panelBottomIndicatorRegistry {
		if spec.Edge != PanelBottomEdgeEnd || spec.ID == PanelBottomIndicatorOtherPanel {
			continue
		}
		if !panelBottomIndicatorVisible(spec.ID, ctx) {
			continue
		}
		fixed += utf8.RuneCountInString(panelBottomIndicatorLabel(spec.ID, ctx))
	}
	if hideInactivePanel && panelID == activePanel && otherPanelPath != "" {
		ctx.EndEdgePathMaxRunes = max(0, avail-fixed-4)
	} else {
		ctx.EndEdgePathMaxRunes = max(0, avail-fixed)
	}
	return ctx
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

// drawPanelBottomEndEdgeIndicators paints End-edge registry segments (sync, quick view, hidden other path).
func drawPanelBottomEndEdgeIndicators(screen tcell.Screen, rect Rect, panelID int, ctx PanelBottomIndicatorContext) {
	if ctx.ChromeBlocked {
		return
	}
	segs := panelBottomEndEdgeSegments(ctx)
	if len(segs) == 0 {
		return
	}
	available := rect.Width - 2
	totalW := panelBottomEndEdgeTotalWidth(ctx)
	if totalW > available {
		segs = dropPanelBottomIndicatorsForWidth(segs, available, false)
	}
	if len(segs) == 0 {
		return
	}
	y := rect.Y + rect.Height - 1
	if panelID == RightPanel {
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
	// Left panel: anchor at physical right; higher Order is rightmost (at the corner).
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

	drawPanelBottomStartEdgeIndicators(screen, rect, ctx.PanelID, y, lastIn, startEdge, ctx.BorderStyle)

	if len(physicalLeft) > 0 {
		x := panelBottomPhysicalLeftChainStartX(rect, ctx.SelectionsBottomHint)
		if x <= endX {
			leadingDash := !ctx.SelectionsBottomHint
			maxCols := endX - x + 1
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
}

// drawPanelBottomStartEdgeIndicators paints corner-anchored segments (Selections).
func drawPanelBottomStartEdgeIndicators(screen tcell.Screen, rect Rect, panelID, y, lastIn int, segs []panelBottomIndicatorSegment, borderStyle tcell.Style) {
	if len(segs) == 0 {
		return
	}
	available := rect.Width - 2
	segs = dropPanelBottomIndicatorsForWidth(segs, available, true)
	for _, seg := range segs {
		padW := utf8.RuneCountInString(seg.Label)
		need := 1 + padW
		if need > available {
			continue
		}
		if panelID == RightPanel {
			xTitle := lastIn - padW
			primitive.TextOverlay(screen, xTitle, y, padW, seg.Label, seg.Style)
			screen.SetContent(lastIn, y, '─', nil, borderStyle)
			continue
		}
		x0 := rect.X + 1
		screen.SetContent(x0, y, '─', nil, borderStyle)
		primitive.TextOverlay(screen, x0+1, y, padW, seg.Label, seg.Style)
	}
}
