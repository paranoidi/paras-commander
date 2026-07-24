package dialog

import (
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// DialogInputRestoreFooterEligible reports whether the footer should show the preferred
// restore-default shortcut: a focused dialog text field (file dialog, transfer dialog, or SFTP
// connect dialog) whose Prefill is non-empty (same contexts where restore is meaningful).
func (h *Handler) DialogInputRestoreFooterEligible() bool {
	if h.keysDialogInput == nil {
		return false
	}
	if lbl := h.keysDialogInput.MenuBindingLabel(keymap.ActionDialogInputRestoreDefault); lbl == "" {
		return false
	}
	if h.model.FileDialog.Open {
		if dialog.FileDialogHasRenamePhase(h.model.FileDialog.DialogType) && h.model.FileDialog.RenamePhase != dialog.RenamePhaseMain {
			return false
		}
		if h.FileDialogOnButton() || h.fileDialogOnRadio() {
			return false
		}
		f := h.FocusedField()
		return f != nil && f.Prefill != ""
	}
	if h.model.TransferDialog.Open {
		d := &h.model.TransferDialog
		if d.FocusField != 0 {
			return false
		}
		switch d.Phase {
		case dialog.TransferPhaseSelfCopyRename:
			return d.SelfCopyNewName.Prefill != ""
		case dialog.TransferPhaseDestination:
			if d.DestSubFocus != dialog.TransferDestSubFocusText {
				return false
			}
			return d.Destination.Prefill != ""
		default:
			return false
		}
	}
	if h.model.SFTPConnectDialog.Open {
		st := &h.model.SFTPConnectDialog
		if st.Focus != 1 {
			return false
		}
		return st.Location.Prefill != ""
	}
	return false
}
