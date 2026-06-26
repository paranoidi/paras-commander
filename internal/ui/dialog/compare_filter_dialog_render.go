package dialog

import (
	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// DrawCompareFilterDialog paints the compare category filter picker modal.
func DrawCompareFilterDialog(screen tcell.Screen, layout Layout, state CompareFilterDialogState, styles theme.Theme) {
	const (
		width  = 32
		height = 11 // border + 6 radios + separator + blank + buttons + border
	)
	rect, ok := draw.ClampCenteredDialogRect(layout, width, height, 24, height)
	if !ok {
		return
	}
	borderStyle := draw.DrawDialogFrame(screen, rect, "Category", styles)

	y := rect.Y + 1

	radios := []struct {
		filter   comparepkg.Filter
		label    string
		shortcut rune
	}{
		{comparepkg.FilterAll, "All", 'A'},
		{comparepkg.FilterEqual, "Equal", 'E'},
		{comparepkg.FilterRelocated, "Relocated", 'R'},
		{comparepkg.FilterPrimaryOnly, "Primary only", 'P'},
		{comparepkg.FilterSecondaryOnly, "Secondary only", 'S'},
		{comparepkg.FilterContentDiff, "Content diff", 'D'},
	}
	selectedFilter, _ := CompareFilterForFocus(state.Focus)
	for i, r := range radios {
		draw.DrawDialogRadio(screen, draw.DialogOptionX(rect), y, r.label, r.shortcut,
			selectedFilter == r.filter, state.Focus == i, styles)
		y++
	}

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	y++ // blank above buttons

	draw.DrawDialogButtonRowCentered(screen, rect, y, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: state.Focus == CompareFilterDialogOKIndex()},
		{Label: "Cancel", Shortcut: 'C', Focused: state.Focus == CompareFilterDialogCancelIndex()},
	}, styles)
}
