// Package filterlist is the App-independent core of fuzzy-list ranking, selection
// clamping, and list-motion keys shared by history, SFTP connect, path picker,
// mass-rename pattern picker, run-for-each history, pin, and the preview theme picker.
package filterlist

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
)

// SyncRanks ranks display lines against query and builds ranked indices plus
// per-slot match ranges (indexed by original line index).
func SyncRanks(lines []string, query string, matchRangeSlots int, caseInsensitive bool) (ranked []int, matchRanges [][]search.Range) {
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

// ClampSelection keeps *selected in [0, rankedLen-1], or 0 when the list is empty.
func ClampSelection(selected *int, rankedLen int) {
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

// ClampedDelta returns selected+delta clamped to [0, rankedLen-1]. If rankedLen <= 0 it returns 0.
func ClampedDelta(selected, rankedLen, delta int) int {
	if rankedLen <= 0 {
		return 0
	}
	return min(max(selected+delta, 0), rankedLen-1)
}

// NavKey computes the new selected index for the list-navigation keys shared by fuzzy-picker
// style dialogs (Up/Down by one row, PgUp/PgDn by pageSize rows, Ctrl+Home/Ctrl+End to the
// first/last row). pageSize is clamped to at least 1. ok is false when rankedLen <= 0 or key is
// not one of these keys (or is Home/End without Ctrl): the caller leaves selected unchanged and
// falls through to its own handling.
func NavKey(key tcell.Key, mods tcell.ModMask, selected, rankedLen, pageSize int) (newSelected int, ok bool) {
	if rankedLen <= 0 {
		return selected, false
	}
	switch key {
	case tcell.KeyUp:
		return ClampedDelta(selected, rankedLen, -1), true
	case tcell.KeyDown:
		return ClampedDelta(selected, rankedLen, 1), true
	case tcell.KeyPgUp:
		return ClampedDelta(selected, rankedLen, -max(1, pageSize)), true
	case tcell.KeyPgDn:
		return ClampedDelta(selected, rankedLen, max(1, pageSize)), true
	case tcell.KeyHome:
		if mods&tcell.ModCtrl != 0 {
			return 0, true
		}
	case tcell.KeyEnd:
		if mods&tcell.ModCtrl != 0 {
			return rankedLen - 1, true
		}
	}
	return selected, false
}

// HandleSelectionKey handles list motion when focus is on the filtered list (focus index 0).
func HandleSelectionKey(ev *tcell.EventKey, focus int, selected *int, rankedLen int, listRows func() int, ensureScroll func()) bool {
	if focus != 0 || rankedLen <= 0 {
		return false
	}
	sel, ok := NavKey(ev.Key(), ev.Modifiers(), *selected, rankedLen, listRows()-1)
	if !ok {
		return false
	}
	*selected = sel
	ensureScroll()
	return true
}
