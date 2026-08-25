package ui

import comparepkg "github.com/paranoidi/paras-commander/internal/compare"

// CompareViewState tracks focus and scrolling for the Compare screen.
type CompareViewState struct {
	Selected    int
	ListScroll  int
	Filter      comparepkg.Filter
	FocusColumn CompareColumnFocus
	IgnoreEmpty bool // hide zero-byte rows (default true on open)
}

// CompareColumnFocus is which path column has keyboard focus for selection.
type CompareColumnFocus int

const (
	CompareColumnPrimary CompareColumnFocus = iota
	CompareColumnSecondary
)
