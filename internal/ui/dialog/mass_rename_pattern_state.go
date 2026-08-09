package dialog

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/search"
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
	Focus       int // 0=list+query, 1=OK, 2=Cancel
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
