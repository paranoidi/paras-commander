package dialog

import (
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// setSortGoldenFocus points st.Focus at a named position. It is the only golden-test code that
// knows how SortDialogState encodes focus.
func setSortGoldenFocus(st *SortDialogState, pos sortPos) {
	spec := SortDialogView(*st)
	var ids []dlg.ID
	switch pos.kind {
	case "radio":
		ids = itemIDs(spec.Groups[0].Columns[0])
	case "meta":
		ids = itemIDs(spec.Groups[0].Columns[1])
	case "cb":
		ids = itemIDs(spec.Groups[1].Columns[0])
	case "ok":
		st.Focus = dlg.OK
		return
	case "cancel":
		st.Focus = dlg.Cancel
		return
	}
	st.Focus = ids[pos.idx]
}

func itemIDs(items []dlg.Item) []dlg.ID {
	ids := make([]dlg.ID, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids
}
