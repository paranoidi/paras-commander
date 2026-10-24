package meta

import (
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func (h *Handler) resolveEditPath(panelID int) (string, error) {
	metaPath, warns := metacmds.ResolveMetaTOML(h.config, h.model.UserHomeDir, h.configDir, h.host.PanelByID(panelID).PathString())
	for _, w := range warns {
		h.host.SetTransientMessage(w, ui.MessageUrgencyWarn)
	}
	if metaPath == "" {
		return h.ensureGlobalStub()
	}
	return metaPath, nil
}

func (h *Handler) editConfigFromDialog() {
	st := &h.model.MetaDialog
	if !st.Open {
		return
	}
	path, err := h.resolveEditPath(st.PanelID)
	if err != nil {
		h.host.SetErrorMessage("Meta commands", err)
		return
	}
	if h.OpenFileEditor(path) {
		h.refreshDialogAfterConfigEdit()
	}
}

func (h *Handler) refreshDialogAfterConfigEdit() {
	st := &h.model.MetaDialog
	if !st.Open {
		return
	}
	panelID := st.PanelID
	prevChecked := make(map[string]bool, len(st.Entries))
	for i, e := range st.Entries {
		if i < len(st.Checked) && st.Checked[i] {
			prevChecked[e.Name] = true
		}
	}
	prevFocusName := focusedEntryName(st)
	prevFocus := st.Focus

	mf := h.loadMetaFile(panelID)
	entries := entriesFromFile(mf)
	checked := make([]bool, len(entries))
	for i, e := range entries {
		checked[i] = prevChecked[e.Name]
	}

	st.Entries = entries
	st.Checked = checked
	st.Focus = refocusAfterEdit(entries, prevFocus, prevFocusName)
}

func entryIndexByName(entries []dialog.MetaEntry, name string) int {
	if name == "" {
		return -1
	}
	for i, e := range entries {
		if e.Name == name {
			return i
		}
	}
	return -1
}

// focusedEntryName is the name of the entry row the dialog focus is on, "" when it is on a button.
func focusedEntryName(st *dialog.MetaDialogState) string {
	for i, e := range st.Entries {
		if st.Focus == dialog.MetaEntryID(i) {
			return e.Name
		}
	}
	return ""
}

// refocusAfterEdit keeps a focused button, follows a focused entry to its new row by name, and
// otherwise falls back to the first row.
func refocusAfterEdit(entries []dialog.MetaEntry, prev dlg.ID, prevName string) dlg.ID {
	if prev == dlg.OK || prev == dlg.Cancel {
		return prev
	}
	if i := entryIndexByName(entries, prevName); i >= 0 {
		return dialog.MetaEntryID(i)
	}
	return ""
}
