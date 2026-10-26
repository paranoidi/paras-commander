package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Widget IDs of the Pin dialog. The list is driven by the filter input and has no focus of its own.
const (
	PinFilter dlg.ID = "pin-filter"
	PinList   dlg.ID = "pin-list"
)

// pinMinWidth is the narrowest terminal-clamped dialog width that still draws.
const pinMinWidth = 36

// PinDialogListRows returns the Pin dialog's list height on layout, or 0 when the dialog does not
// fit.
func PinDialogListRows(layout Layout) int {
	return fittedListHeight(pinFrame(PinDialogState{}, layout, theme.Theme{}, ""), layout, PinList)
}

// pinList is the pin list item: the ranked items with their fuzzy highlights, icons and
// missing-path variant. Rows whose item index is out of range stay blank so row indices keep
// matching state.Ranked.
func pinList(state PinDialogState, items []PinDialogItem, listRows int, styles theme.Theme) dlg.Item {
	list := dlg.Item{
		ID: PinList, Kind: dlg.KindList, Driven: true, Fit: dlg.FitPath, Scrollbar: true,
		Cursor: state.Selected, Offset: state.ListScroll, Height: listRows,
	}
	for _, ent := range state.Ranked {
		var row dlg.ListRow
		if ent >= 0 && ent < len(items) {
			it := items[ent]
			row = dlg.ListRow{Text: it.Path, Missing: it.PathMissing, Icon: styles.IconFile()}
			if it.IsDir {
				row.Icon = styles.IconFolder()
			}
			if ent < len(state.MatchRanges) {
				row.Match = state.MatchRanges[ent]
			}
		}
		list.Rows = append(list.Rows, row)
	}
	return list
}

// EnsurePinListScroll keeps Selected in range and visible in the pin list of height listRows.
func EnsurePinListScroll(state *PinDialogState, listRows int) {
	EnsurePickerScroll(len(state.Ranked), &state.Selected, &state.ListScroll, listRows)
}

// PinDialogView describes the Pin dialog: a filter input that steers the pin list below it, a
// blank row and no buttons. The filter always holds focus.
func PinDialogView(state PinDialogState, items []PinDialogItem, layout Layout, styles theme.Theme, scrollbarStyle uiscrollbar.Style) dlg.Spec {
	spec, _ := fillList(pinFrame(state, layout, styles, scrollbarStyle), layout, PinList, func(height, _ int) dlg.Item {
		return pinList(state, items, height, styles)
	})
	return spec
}

// pinFrame is the Pin spec with an empty list as tall as a picker list wants. There is no button
// strip: Esc closes, and the open-in-panel, remove and Enter actions cover what buttons would.
func pinFrame(state PinDialogState, layout Layout, styles theme.Theme, scrollbarStyle uiscrollbar.Style) dlg.Spec {
	filter := dlg.Item{
		ID: PinFilter, Kind: dlg.KindInput, Drives: PinList, LeadingIcon: styles.IconSearchIcon(),
		Value: state.Query, Cursor: state.QueryCursor, Scroll: state.QueryScroll,
	}
	return dlg.Spec{
		Title: "Pin", Width: min(78, layout.Width-4), MinWidth: pinMinWidth,
		ScrollbarStyle: scrollbarStyle,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			filter, {Kind: dlg.KindSeparator},
			{ID: PinList, Kind: dlg.KindList, Height: pickerListHeight(layout.Height), MinHeight: pickerMinListRows},
			{Kind: dlg.KindLabel},
		}}}},
	}
}

// DrawPinDialog paints the Pin dialog: a fuzzy-filterable, most-recently-pinned-first list of
// pinned files and directories. There are no buttons; Esc closes.
func DrawPinDialog(screen tcell.Screen, layout Layout, state PinDialogState, items []PinDialogItem, styles theme.Theme, scrollbarStyle uiscrollbar.Style) {
	dlg.Draw(screen, layout.Width, layout.Height, PinDialogView(state, items, layout, styles, scrollbarStyle), PinFilter, styles)
}
