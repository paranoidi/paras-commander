package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// tryMkdirDialogShortcut handles [dialog.mkdir] while the mkdir dialog is open.
// Returns true when the event was consumed.
func (a *App) tryMkdirDialogShortcut(ev *tcell.EventKey) bool {
	if a.keysMkdirDialog == nil {
		return false
	}
	d := &a.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogMkdir {
		return false
	}
	id, ok := a.keysMkdirDialog.Lookup(ev)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionFileMkdirExtractCommonName:
		return a.applyMkdirExtractCommonName()
	default:
		return false
	}
}

func (a *App) mkdirDialogExtractFooterEligible() bool {
	if a.keysMkdirDialog == nil {
		return false
	}
	d := &a.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogMkdir {
		return false
	}
	if len(a.activePanel().SelectedPaths) < 2 {
		return false
	}
	return a.keysMkdirDialog.MenuBindingLabel(keymap.ActionFileMkdirExtractCommonName) != ""
}

func (a *App) applyMkdirExtractCommonName() bool {
	p := a.activePanel()
	source, err := ops.ResolveSource(p)
	if err != nil || len(source.Entries) < 2 {
		a.setErrorMessage("Mkdir", ops.SourceError("select at least two entries to extract a common name"))
		return true
	}
	names := make([]string, len(source.Entries))
	for i, e := range source.Entries {
		names[i] = e.Name
	}
	extracted := dialog.ExtractLongestCommonName(names)
	if extracted == "" {
		a.setErrorMessage("Mkdir", ops.SourceError("no common name found in selection"))
		return true
	}
	d := &a.model.FileDialog
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
