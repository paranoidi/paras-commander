package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// tryMkdirDialogShortcut handles [dialog.mkdir] while the mkdir dialog is open.
// Returns true when the event was consumed.
func (h *Handler) tryMkdirDialogShortcut(ev *tcell.EventKey) bool {
	if h.keysMkdirDialog == nil {
		return false
	}
	d := &h.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogMkdir {
		return false
	}
	id, ok := h.keysMkdirDialog.Lookup(ev)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionFileMkdirExtractCommonName:
		return h.applyMkdirExtractCommonName()
	default:
		return false
	}
}

// MkdirDialogExtractFooterEligible reports whether the footer should show the "Extract common
// name" shortcut hint.
func (h *Handler) MkdirDialogExtractFooterEligible() bool {
	if h.keysMkdirDialog == nil {
		return false
	}
	d := &h.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogMkdir {
		return false
	}
	if len(h.host.ActivePanel().SelectedPaths) < 2 {
		return false
	}
	return h.keysMkdirDialog.MenuBindingLabel(keymap.ActionFileMkdirExtractCommonName) != ""
}

func (h *Handler) applyMkdirExtractCommonName() bool {
	p := h.host.ActivePanel()
	source, err := ops.ResolveSource(p)
	if err != nil || len(source.Entries) < 2 {
		h.host.SetErrorMessage("Mkdir", ops.SourceError("select at least two entries to extract a common name"))
		return true
	}
	names := make([]string, len(source.Entries))
	for i, e := range source.Entries {
		names[i] = e.Name
	}
	extracted := dialog.ExtractLongestCommonName(names)
	if extracted == "" {
		h.host.SetErrorMessage("Mkdir", ops.SourceError("no common name found in selection"))
		return true
	}
	d := &h.model.FileDialog
	if len(d.Fields) < 1 {
		return true
	}
	f := &d.Fields[0]
	f.Value = extracted
	f.Prefill = extracted
	f.Cursor = len([]rune(extracted))
	f.PrefillPending = true
	return true
}
