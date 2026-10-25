package dialog

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// MassRenamePatternPickerState is a fuzzy-filtered list of ops.MassRenamePattern entries. It
// backs two separate dialog phases with the same widget shape: MassRenameLoadPicker (saved
// patterns from patterns.toml, MassRenamePhase == MassRenamePhaseLoadPicker) and
// MassRenameHistoryPicker (in-memory recently-used patterns, MassRenamePhase ==
// MassRenamePhaseHistoryPicker).
type MassRenamePatternPickerState struct {
	Items       []ops.MassRenamePattern
	Query       string
	QueryCursor int
	QueryScroll int
	Ranked      []int
	MatchRanges [][]search.Range
	Selected    int
	ListScroll  int
	Focus       dlg.ID // PickerFilter (also the empty value), dlg.OK or dlg.Cancel
}

// MassRenamePickerPhase reports whether phase is one of the three sub-screens rendered as a
// fuzzy pattern picker (load, history, overwrite). Single source of truth for the
// picker-vs-form branches in rendering, focus indexing and key handling.
func MassRenamePickerPhase(phase MassRenamePhase) bool {
	switch phase {
	case MassRenamePhaseLoadPicker, MassRenamePhaseHistoryPicker, MassRenamePhaseOverwritePicker:
		return true
	default:
		return false
	}
}

// MassRenamePatternSearchLine returns the fuzzy-search corpus and display line for one
// load/history-picker entry. Saved patterns always have a Name and use "Name Description";
// history entries are unnamed and fall back to a "Find → Replace" summary ("Capitalize" in
// capitalize mode, which has no Find/Replace).
func MassRenamePatternSearchLine(p ops.MassRenamePattern) string {
	if strings.TrimSpace(p.Name) != "" {
		return strings.TrimSpace(p.Name + " " + p.Description)
	}
	if p.Mode == "capitalize" {
		return "Capitalize"
	}
	return strings.TrimSpace(p.Find + " → " + p.Replace)
}

// EnsureMassRenamePatternPickerListScroll keeps Selected in range and visible in a list of height
// listRows. Shared by the load, history and overwrite pickers.
func EnsureMassRenamePatternPickerListScroll(state *MassRenamePatternPickerState, listRows int) {
	EnsurePickerScroll(len(state.Ranked), &state.Selected, &state.ListScroll, listRows)
}
