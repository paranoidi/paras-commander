package dialog

import (
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// helpVisualRowIndex returns the visual-row index of the entry row whose RankedIdx == selected.
// Returns 0 if not found (should not happen for a valid selected index).
func helpVisualRowIndex(rows []HelpVisualRow, selected int) int {
	for i, r := range rows {
		if !r.IsHeader && r.RankedIdx == selected {
			return i
		}
	}
	return 0
}

// EnsureListScroll clamps Selected to the ranked entries and sets ListScroll (the first visible
// *visual* row, counting section headers as well as entries) to the list's viewport for a list of
// listRows rows: dlg's sticky viewport, which also reveals the section header right above the
// selected entry.
func (st *HelpViewState) EnsureListScroll(listRows int) {
	n := len(st.Ranked)
	if n == 0 || listRows <= 0 {
		st.ListScroll = 0
		return
	}
	st.Selected = min(max(st.Selected, 0), n-1)
	st.ListScroll = helpListItem(*st, listRows, 0, theme.Theme{}).ViewOffset()
}

// helpLandOnEntry adjusts a visual-row index that may have landed on a non-selectable section
// header onto the nearest actual entry row. preferBackward true steps toward the start of the
// list (used when the caller must not travel further forward than intended — e.g. snapping to
// the bottom of the current view, where stepping onto the header's own entry would spill onto
// the next, not-yet-visible section); preferBackward false steps toward the end. Row 0 is
// always a header (BuildHelpVisualRows starts every list with one), so there's nothing to step
// back to there — preferBackward is ignored at that edge.
func helpLandOnEntry(rows []HelpVisualRow, target int, preferBackward bool) int {
	if !rows[target].IsHeader {
		return target
	}
	if preferBackward && target > 0 {
		return target - 1
	}
	return target + 1
}

// PageDownSelection returns the Ranked index to select for PgDn. If the current selection
// isn't already the last entry visible in [ListScroll, ListScroll+listRows), it jumps straight
// to that entry without needing to scroll — PgDn always selects the last already-visible item
// first, only scrolling once the selection is already there, at which point it advances a full
// page and lands on the bottom entry of the next page.
func (st *HelpViewState) PageDownSelection(listRows int) int {
	rows := BuildHelpVisualRows(st.Entries, st.Ranked)
	if len(rows) == 0 {
		return st.Selected
	}
	row := helpVisualRowIndex(rows, st.Selected)
	bottom := st.ListScroll + listRows - 1
	if bottom >= len(rows) {
		bottom = len(rows) - 1
	}
	bottomEntry := helpLandOnEntry(rows, bottom, true)
	if row >= bottomEntry {
		bottom = row + listRows
		if bottom >= len(rows) {
			bottom = len(rows) - 1
		}
		bottomEntry = helpLandOnEntry(rows, bottom, true)
	}
	return rows[bottomEntry].RankedIdx
}

// PageUpSelection is PageDownSelection's mirror for PgUp: it jumps to the first entry visible
// in the current view, or — once already there — pages a full screen further back.
func (st *HelpViewState) PageUpSelection(listRows int) int {
	rows := BuildHelpVisualRows(st.Entries, st.Ranked)
	if len(rows) == 0 {
		return st.Selected
	}
	row := helpVisualRowIndex(rows, st.Selected)
	top := st.ListScroll
	if top < 0 {
		top = 0
	}
	topEntry := helpLandOnEntry(rows, top, false)
	if row <= topEntry {
		top = row - listRows
		if top < 0 {
			top = 0
		}
		topEntry = helpLandOnEntry(rows, top, false)
	}
	return rows[topEntry].RankedIdx
}

// ApplyListMove stores a list event of the help entry list: the cursor row (a visual row; header
// rows never hold it) becomes the selected entry, and the viewport its ListScroll.
func (st *HelpViewState) ApplyListMove(ev dlg.Event) {
	visual := BuildHelpVisualRows(st.Entries, st.Ranked)
	if ev.Index >= 0 && ev.Index < len(visual) && !visual[ev.Index].IsHeader {
		st.Selected = visual[ev.Index].RankedIdx
	}
	st.ListScroll = ev.Offset
}
