package dialog

import "github.com/paranoidi/paras-commander/internal/search"

// RunForEachHistoryPickerState is a fuzzy-filtered list of in-memory recently-run run-for-each
// command lines (most-recent-first). Structurally near-identical to MassRenamePatternPickerState
// (mass_rename_pattern_dialog_render.go) — both back a fuzzy-filtered in-memory-list picker —
// differing only in item type (plain string vs ops.MassRenamePattern); a future cleanup could
// consolidate the two behind a generic picker if a third caller ever needs the same shape.
type RunForEachHistoryPickerState struct {
	Items       []string
	Query       string
	QueryCursor int
	QueryScroll int
	Ranked      []int
	MatchRanges [][]search.Range
	Selected    int
	ListScroll  int
	Focus       int // 0=list+query, 1=OK, 2=Cancel
}

// EnsureRunForEachHistoryPickerListScroll keeps Selected row visible in a list of height
// listRows. Mirrors EnsureMassRenamePatternPickerListScroll (mass_rename_pattern_dialog_render.go).
func EnsureRunForEachHistoryPickerListScroll(state *RunForEachHistoryPickerState, listRows int) {
	n := len(state.Ranked)
	if n == 0 || listRows <= 0 {
		state.ListScroll = 0
		return
	}
	if state.Selected < 0 {
		state.Selected = 0
	}
	if state.Selected >= n {
		state.Selected = n - 1
	}
	if state.ListScroll > state.Selected {
		state.ListScroll = state.Selected
	}
	if state.Selected >= state.ListScroll+listRows {
		state.ListScroll = state.Selected - listRows + 1
	}
}
