package meta

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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
	prevFocusName, prevFocusButton := dialogFocusTarget(st.Entries, st.Focus)

	mf := h.loadMetaFile(panelID)
	entries := entriesFromFile(mf)
	checked := make([]bool, len(entries))
	for i, e := range entries {
		checked[i] = prevChecked[e.Name]
	}

	n := len(entries)
	form := dialog.NewDialogLinearForm(n)
	focus := dialogFocusFromTarget(entries, form, prevFocusName, prevFocusButton)

	st.Entries = entries
	st.Checked = checked
	st.Focus = focus
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

func dialogFocusTarget(entries []dialog.MetaEntry, focus int) (entryName string, button tcell.Key) {
	n := len(entries)
	form := dialog.NewDialogLinearForm(n)
	switch {
	case focus < n:
		return entries[focus].Name, 0
	case focus == form.OKIndex():
		return "", tcell.KeyEnter
	case focus == form.CancelIndex():
		return "", tcell.KeyEsc
	default:
		return "", 0
	}
}

func dialogFocusFromTarget(entries []dialog.MetaEntry, form dialog.DialogLinearForm, entryName string, button tcell.Key) int {
	switch button {
	case tcell.KeyEnter:
		return form.OKIndex()
	case tcell.KeyEsc:
		return form.CancelIndex()
	}
	if entryName != "" {
		if i := entryIndexByName(entries, entryName); i >= 0 {
			return i
		}
	}
	return 0
}
