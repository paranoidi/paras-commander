package dialog

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/pathpick"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// navigateToPrefill is the Navigate to dialog's starting text: the action's default key types
// "/", so the field starts as the root the user is about to complete from.
const navigateToPrefill = "/"

// OpenNavigateToDialog opens the Navigate to dialog: one directory path input prefilled with
// "/" (editable in place, with directory completion). OK navigates the active panel there.
func (h *Handler) OpenNavigateToDialog() {
	h.model.FileDialog = dialog.FileDialogState{
		Open:       true,
		DialogType: dialog.FileDialogNavigateTo,
		Fields: []dialog.FileDialogField{{
			Label:              "Path",
			Value:              navigateToPrefill,
			Cursor:             len([]rune(navigateToPrefill)),
			PathPicker:         true,
			CompletionDirsOnly: true,
		}},
	}
	h.SyncFocusedFileDialogPathFieldCompletion()
}

// executeNavigateTo navigates the active panel to the typed path (a leading ~ against home, a
// relative path against the panel directory, sftp:// and fileindex:// URIs as given). A path that
// does not parse keeps the dialog open; the listing itself loads asynchronously, so a missing
// directory reports through the panel's load error after the dialog closed (as bookmarks do).
func (h *Handler) executeNavigateTo() {
	d := &h.model.FileDialog
	if len(d.Fields) < 1 || strings.TrimSpace(d.Fields[0].Value) == "" {
		return
	}
	p := h.host.ActivePanel()
	loc, err := pathpick.ResolveDestination(p.Path, h.model.UserHomeDir, d.Fields[0].Value)
	if err != nil {
		h.host.SetErrorMessage("Navigate to", err)
		return
	}
	if err := h.host.NavigatePanelToPath(h.model.ActivePanel, loc.String(), ""); err != nil {
		h.host.SetErrorMessage("Navigate to", err)
		return
	}
	p.EnsureCursorVisible(h.host.ActiveViewportRows())
	h.CloseFileDialog()
	h.host.SetTransientMessage(loc.String(), ui.MessageUrgencyInfo)
}
