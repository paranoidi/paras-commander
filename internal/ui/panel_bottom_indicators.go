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
	PanelBottomIndicatorSelections PanelBottomIndicatorID = "selections"
	PanelBottomIndicatorGitignore  PanelBottomIndicatorID = "gitignore"
	// PanelBottomIndicatorSync is reserved for a future End-edge registration (drawn separately today).
	PanelBottomIndicatorSync PanelBottomIndicatorID = "sync"
	// PanelBottomIndicatorStash is registered in phase 2.
	PanelBottomIndicatorStash PanelBottomIndicatorID = "stash"
)

// PanelBottomEdge names which horizontal edge of the panel bottom row an indicator uses.
type PanelBottomEdge int

const (
	// PanelBottomEdgeStart is the panel-relative start corner (physical left on LeftPanel,
	// physical right on RightPanel). Used for cross-directory Selections.
	PanelBottomEdgeStart PanelBottomEdge = iota
	// PanelBottomEdgePhysicalLeft chains segments from the physical left interior column
	// (Gitignore and trailing frame dashes on both panels).
	PanelBottomEdgePhysicalLeft
	// PanelBottomEdgeEnd is the panel-relative end corner (Sync today). Not drawn here yet.
	PanelBottomEdgeEnd
)

// PanelBottomIndicatorContext carries panel chrome inputs for visibility and styling.
type PanelBottomIndicatorContext struct {
	PanelID              int
	State                panel.State
	SelectionsBottomHint bool
	SyncDriverPanelID    int
	FileListActive       bool
	ChromeBlocked        bool
	BorderStyle          tcell.Style
	TitleStyle           tcell.Style
	Styles               theme.Theme
}

type panelBottomIndicatorSpec struct {
	ID    PanelBottomIndicatorID
	Edge  PanelBottomEdge
	Order int
}

// panelBottomIndicatorRegistry is the single source of truth for segment order.
var panelBottomIndicatorRegistry = []panelBottomIndicatorSpec{
	{ID: PanelBottomIndicatorSelections, Edge: PanelBottomEdgeStart, Order: 0},
	{ID: PanelBottomIndicatorGitignore, Edge: PanelBottomEdgePhysicalLeft, Order: 0},
	{ID: PanelBottomIndicatorStash, Edge: PanelBottomEdgePhysicalLeft, Order: 1},
}

type panelBottomIndicatorSegment struct {
	ID    PanelBottomIndicatorID
	Edge  PanelBottomEdge
	Order int
	Label string
	Style tcell.Style
}

// panelBottomIndicatorStyle resolves segment paint style. Selections and gitignore follow panel
// title/frame chrome (active/inactive/blocked); other ids use theme panel.bottom.indicator.*.
func panelBottomIndicatorStyle(ctx PanelBottomIndicatorContext, id PanelBottomIndicatorID) tcell.Style {
	switch id {
	case PanelBottomIndicatorSelections:
		return ctx.TitleStyle
	case PanelBottomIndicatorGitignore:
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
	case PanelBottomIndicatorGitignore:
		return ctx.State.GitignoreActive
	case PanelBottomIndicatorStash:
		return ctx.State.StashPathCount() > 0
	default:
		return false
	}
}

func panelBottomIndicatorLabel(id PanelBottomIndicatorID, ctx PanelBottomIndicatorContext) string {
	switch id {
	case PanelBottomIndicatorSelections:
		return panelSelectionsChromePadded
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
	default:
		return ""
	}
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

// panelBottomEndEdgeReservedStart returns the first column (inclusive) still available on the
// bottom interior row before the sync-driver overlay on the End edge.
func panelBottomEndEdgeReservedStart(rect Rect, panelID, syncDriverPanelID int) int {
	lastIn := rect.X + rect.Width - 2
	if syncDriverPanelID != panelID {
		return lastIn
	}
	labelW := utf8.RuneCountInString(panelSyncIndicatorLabel(panelID))
	if labelW > rect.Width-2 {
		return lastIn
	}
	if panelID == RightPanel {
		return rect.X + labelW
	}
	return lastIn - labelW
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

// drawPanelBottomIndicators paints Start-edge and PhysicalLeft-edge registry segments.
func drawPanelBottomIndicators(screen tcell.Screen, rect Rect, ctx PanelBottomIndicatorContext) {
	if rect.Width <= 4 || rect.Height < 2 {
		return
	}
	all := collectPanelBottomIndicators(ctx)
	if len(all) == 0 {
		return
	}
	y := rect.Y + rect.Height - 1
	lastIn := rect.X + rect.Width - 2
	endX := panelBottomEndEdgeReservedStart(rect, ctx.PanelID, ctx.SyncDriverPanelID)

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

	if len(physicalLeft) == 0 {
		return
	}
	x := panelBottomPhysicalLeftChainStartX(rect, ctx.SelectionsBottomHint)
	if x > endX {
		return
	}
	// Legacy: when no selections hint, the chain still opens with a frame dash at physical left.
	leadingDash := !ctx.SelectionsBottomHint
	maxCols := endX - x + 1
	physicalLeft = dropPanelBottomIndicatorsForWidth(physicalLeft, maxCols, leadingDash)
	if len(physicalLeft) == 0 {
		return
	}
	if ctx.SelectionsBottomHint {
		if x > endX {
			return
		}
		screen.SetContent(x, y, '─', nil, ctx.BorderStyle)
		x++
	} else {
		screen.SetContent(x, y, '─', nil, ctx.BorderStyle)
		x++
	}
	for i, seg := range physicalLeft {
		if i > 0 {
			if x > endX {
				return
			}
			screen.SetContent(x, y, '─', nil, ctx.BorderStyle)
			x++
		}
		padW := utf8.RuneCountInString(seg.Label)
		if x+padW-1 > endX {
			return
		}
		primitive.TextOverlay(screen, x, y, padW, seg.Label, seg.Style)
		x += padW
	}
	for xi := x; xi <= endX; xi++ {
		screen.SetContent(xi, y, '─', nil, ctx.BorderStyle)
	}
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
