package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// syncFilteredListRanks ranks display lines against query and builds ranked indices plus
// per-slot match ranges (indexed by original line index).
func syncFilteredListRanks(lines []string, query string, matchRangeSlots int, caseInsensitive bool) (ranked []int, matchRanges [][]search.Range) {
	q := search.Parse(query)
	opts := search.Options{CaseInsensitive: caseInsensitive}
	results := q.Rank(lines, opts)
	ranked = make([]int, len(results))
	matchRanges = make([][]search.Range, matchRangeSlots)
	for i, r := range results {
		ranked[i] = r.Index
		if r.Index >= 0 && r.Index < matchRangeSlots {
			matchRanges[r.Index] = r.Result.Ranges
		}
	}
	return ranked, matchRanges
}

func clampFilteredListSelection(selected *int, rankedLen int) {
	if *selected >= rankedLen {
		if rankedLen == 0 {
			*selected = 0
		} else {
			*selected = rankedLen - 1
		}
	}
	if *selected < 0 {
		*selected = 0
	}
}

// handleFilteredListSelectionKey handles list motion when focus is on the filtered list (focus index 0).
func handleFilteredListSelectionKey(ev *tcell.EventKey, focus int, selected *int, rankedLen int, listRows func() int, ensureScroll func()) bool {
	if focus != 0 || rankedLen <= 0 {
		return false
	}
	rows := listRows()
	switch ev.Key() {
	case tcell.KeyUp:
		*selected = dialog.ListClampedSelectionDelta(*selected, rankedLen, -1)
		ensureScroll()
		return true
	case tcell.KeyDown:
		*selected = dialog.ListClampedSelectionDelta(*selected, rankedLen, 1)
		ensureScroll()
		return true
	case tcell.KeyPgUp:
		step := max(1, rows-1)
		*selected = dialog.ListClampedSelectionDelta(*selected, rankedLen, -step)
		ensureScroll()
		return true
	case tcell.KeyPgDn:
		step := max(1, rows-1)
		*selected = dialog.ListClampedSelectionDelta(*selected, rankedLen, step)
		ensureScroll()
		return true
	case tcell.KeyHome:
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			*selected = 0
			ensureScroll()
			return true
		}
	case tcell.KeyEnd:
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			*selected = rankedLen - 1
			ensureScroll()
			return true
		}
	}
	return false
}
