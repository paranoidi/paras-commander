package meta

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobbridge"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// saveDefaultColumns persists the picker's checked columns as [fileindex].default_columns, at
// most the active column limit of them, so the file holds what will actually be switched on.
// The dialog, its checkboxes and the panel's active columns stay as they are (OK applies).
func (h *Handler) saveDefaultColumns() {
	st := &h.model.MetaDialog
	if !st.Open || len(st.Entries) == 0 {
		return
	}
	names := []string{} // non-nil so an empty selection is written as []
	for i, e := range st.Entries {
		if i < len(st.Checked) && st.Checked[i] {
			names = append(names, e.Name)
		}
	}
	checked := len(names)
	names = names[:min(checked, config.DefaultMetaMaxActiveColumns)]
	if err := h.host.SaveDefaultColumns(names); err != nil {
		h.host.SetErrorMessage("Meta commands", err)
		return
	}
	h.config.FileIndex.DefaultColumns = names
	if checked > len(names) {
		h.host.SetTransientMessage(fmt.Sprintf("Saved first %d of %d default columns", len(names), checked), ui.MessageUrgencyWarn)
		return
	}
	msg := "Saved no default columns"
	if n := len(names); n > 0 {
		msg = fmt.Sprintf("Saved %d default %s", n, jobbridge.Plural(n, "column", "columns"))
	}
	h.host.SetTransientMessage(msg, ui.MessageUrgencySuccess)
}
