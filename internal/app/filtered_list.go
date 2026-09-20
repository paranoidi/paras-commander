package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/filterlist"
	"github.com/paranoidi/paras-commander/internal/search"
)

func syncFilteredListRanks(lines []string, query string, matchRangeSlots int, caseInsensitive bool) (ranked []int, matchRanges [][]search.Range) {
	return filterlist.SyncRanks(lines, query, matchRangeSlots, caseInsensitive)
}

func clampFilteredListSelection(selected *int, rankedLen int) {
	filterlist.ClampSelection(selected, rankedLen)
}

func handleFilteredListSelectionKey(ev *tcell.EventKey, focus int, selected *int, rankedLen int, listRows func() int, ensureScroll func()) bool {
	return filterlist.HandleSelectionKey(ev, focus, selected, rankedLen, listRows, ensureScroll)
}
