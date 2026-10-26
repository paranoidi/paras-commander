package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

const (
	findDialogMinWidth    = 54 // narrower terminals draw no find dialog
	findDialogMinListRows = 4
	findDialogMaxListRows = 26
)

// findListHeight is how many result rows the find dialog wants on a layout of layoutHeight rows
// (4..26, leaving a margin around the dialog); dlg.FitSpec shortens it where the dialog's other
// rows do not leave that many.
func findListHeight(layoutHeight int) int {
	return min(max(layoutHeight-14, findDialogMinListRows), findDialogMaxListRows)
}

// FindDialogMetrics returns the find dialog's width, height and visible list row count, from its
// placed spec. listH is the result list's height in FindDialogView and is used for list scroll
// math; ok is false when the dialog does not fit.
func FindDialogMetrics(layout Layout, showSearchSelectionsOption bool) (width, height, listH int, ok bool) {
	spec := FindDialogView(&FindDialogState{ShowSearchSelectionsOption: showSearchSelectionsOption}, FindViewContext{Layout: layout})
	p, ok := dlg.Place(layout.Width, layout.Height, spec)
	if !ok {
		return 0, 0, 0, false
	}
	list, _ := spec.Item(FindList)
	return p.Rect.Width, p.Rect.Height, list.Height, true
}
