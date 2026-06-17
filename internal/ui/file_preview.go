package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// Preview types re-exported from previewpanel (single source for scrollable preview state).
type (
	FilePreviewState = previewpanel.State
	FilePreviewPhase = previewpanel.Phase
	PreviewSource    = previewpanel.Source
	AnsiCell         = previewpanel.AnsiCell
)

const (
	FilePreviewPhaseIdle    = previewpanel.PhaseIdle
	FilePreviewPhasePending = previewpanel.PhasePending
	FilePreviewPhaseRunning = previewpanel.PhaseRunning
	FilePreviewPhaseDone    = previewpanel.PhaseDone

	PreviewSourceExternalANSI        = previewpanel.SourceExternalANSI
	PreviewSourceInternalHighlighted = previewpanel.SourceInternalHighlighted
)

// AnsiStyledCells expands ANSI SGR sequences into styled cells.
func AnsiStyledCells(s string, base tcell.Style) []AnsiCell {
	return previewpanel.AnsiStyledCells(s, base)
}

// WrapAnsiCells breaks cells into visual lines.
func WrapAnsiCells(cells []AnsiCell, width int) [][]AnsiCell {
	return previewpanel.WrapAnsiCells(cells, width)
}

// FilePreviewBodyStyle is the base text style for preview body rows.
func FilePreviewBodyStyle(styles theme.Theme, chromeBlocked bool) tcell.Style {
	bg := auxPanelContentBG(styles, chromeBlocked)
	return auxPanelBodyText(styles, chromeBlocked, bg)
}

// FilePreviewHoldable reports whether st has displayable body content worth keeping while loading.
func FilePreviewHoldable(st FilePreviewState) bool {
	return st.Holdable()
}

// FilePreviewDrawWarmCandidate is true when wrapped lines should be precomputed for drawing.
func FilePreviewDrawWarmCandidate(st FilePreviewState) bool {
	return st.DrawWarmCandidate()
}

// MergeFilePreviewDrawWithHold builds a draw snapshot that keeps the previous preview body visible
// while live is loading a new path.
func MergeFilePreviewDrawWithHold(live, hold FilePreviewState) FilePreviewState {
	return previewpanel.MergeDrawWithHold(live, hold)
}

// FilePreviewTotalLines returns how many wrapped lines the preview body would use for vertical scrolling.
func FilePreviewTotalLines(st FilePreviewState, textWidth int, base tcell.Style) int {
	return previewpanel.TotalLines(st, textWidth, base)
}
