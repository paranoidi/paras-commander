package ui

import comparepkg "github.com/paranoidi/paras-commander/internal/compare"

// CompareViewState tracks focus and scrolling for the Compare screen.
type CompareViewState struct {
	Selected    int
	ListScroll  int
	Filter      comparepkg.Filter
	FocusColumn CompareColumnFocus
	IgnoreEmpty bool // hide zero-byte rows (default true on open)
	// NameFilter, when non-nil, keeps only rows whose basename it accepts (set by the Filter
	// dialog); NameFilterLabel is its border label.
	NameFilter      func(name string) bool
	NameFilterLabel string
	// RangeAnchor is the focused column's absolute path marked as the pending range-select
	// start ("" when none); cleared on column change and refresh.
	RangeAnchor string
	// SwapPanels is set when the main panels were swapped on open: the left compare column
	// then belongs to the Secondary panel (see PanelID).
	SwapPanels bool
}

// PanelID returns the main-panel ID that owns the left (secondary=false) or right column.
func (s CompareViewState) PanelID(secondary bool) int {
	if secondary != s.SwapPanels {
		return SecondaryPanel
	}
	return PrimaryPanel
}

// CompareColumnFocus is which path column has keyboard focus for selection.
type CompareColumnFocus int

const (
	CompareColumnPrimary CompareColumnFocus = iota
	CompareColumnSecondary
)
