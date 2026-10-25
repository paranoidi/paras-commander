package dialog

import (
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

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
	Focus       dlg.ID // PickerFilter (also the empty value), dlg.OK or dlg.Cancel
}

// EnsureRunForEachHistoryPickerListScroll keeps Selected in range and visible in a list of height
// listRows.
func EnsureRunForEachHistoryPickerListScroll(state *RunForEachHistoryPickerState, listRows int) {
	EnsurePickerScroll(len(state.Ranked), &state.Selected, &state.ListScroll, listRows)
}

// EnsurePickerScroll clamps *selected into n rows and sets *offset to the list offset that keeps it
// visible in a viewport of listRows.
func EnsurePickerScroll(n int, selected, offset *int, listRows int) {
	if n == 0 || listRows <= 0 {
		*offset = 0
		return
	}
	*selected = min(max(*selected, 0), n-1)
	*offset = dlg.Item{Kind: dlg.KindList, Rows: make([]dlg.ListRow, n), Cursor: *selected, Offset: *offset, Height: listRows}.ViewOffset()
}
