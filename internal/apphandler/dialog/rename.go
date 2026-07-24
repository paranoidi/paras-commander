package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// tryRenameDialogShortcut handles [dialog.rename] while the main
// rename dialog (name field) is active. Returns true when the event was consumed.
func (h *Handler) tryRenameDialogShortcut(ev *tcell.EventKey) bool {
	if h.keysRenameDialog == nil {
		return false
	}
	d := &h.model.FileDialog
	if !d.Open || !dialog.FileDialogHasRenamePhase(d.DialogType) || d.RenamePhase != dialog.RenamePhaseMain {
		return false
	}
	id, ok := h.keysRenameDialog.Lookup(ev)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionFileRenameOpenSanitize:
		d.RenamePhase = dialog.RenamePhaseSanitize
		d.RenameSanitizeDots = true
		d.RenameSanitizeUnderscores = true
		d.FocusedField = dialog.FileDialogOKFocusIndex(*d)
		return true
	case keymap.ActionFileRenameOpenSlugify:
		d.RenamePhase = dialog.RenamePhaseSlugify
		d.RenameSlugifySep = dialog.RenameSlugifyDot
		d.FocusedField = dialog.FileDialogOKFocusIndex(*d)
		return true
	case keymap.ActionFileRenameOpenEncoding:
		if len(d.RenameEncodingCandidates) == 0 {
			return false
		}
		d.RenamePhase = dialog.RenamePhaseEncoding
		if d.RenameEncodingSelected < 0 || d.RenameEncodingSelected >= len(d.RenameEncodingCandidates) {
			d.RenameEncodingSelected = 0
		}
		d.FocusedField = dialog.FileDialogOKFocusIndex(*d)
		return true
	default:
		return false
	}
}

// RenameDialogFooterEligible reports whether the footer should show the rename dialog's
// tool-shortcut hints (Sanitize/Slugify/Encoding).
func (h *Handler) RenameDialogFooterEligible() bool {
	if h.keysRenameDialog == nil {
		return false
	}
	d := &h.model.FileDialog
	if !d.Open || !dialog.FileDialogHasRenamePhase(d.DialogType) || d.RenamePhase != dialog.RenamePhaseMain {
		return false
	}
	return h.keysRenameDialog.MenuBindingLabel(keymap.ActionFileRenameOpenSanitize) != "" ||
		h.keysRenameDialog.MenuBindingLabel(keymap.ActionFileRenameOpenSlugify) != "" ||
		(len(d.RenameEncodingCandidates) > 0 && h.keysRenameDialog.MenuBindingLabel(keymap.ActionFileRenameOpenEncoding) != "")
}

// RenameEncodingFooterEligible reports whether the footer should show the Encoding tool
// shortcut specifically (a subset of RenameDialogFooterEligible's conditions).
func (h *Handler) RenameEncodingFooterEligible() bool {
	d := &h.model.FileDialog
	return d.Open && dialog.FileDialogHasRenamePhase(d.DialogType) && d.RenamePhase == dialog.RenamePhaseMain &&
		len(d.RenameEncodingCandidates) > 0 && h.keysRenameDialog != nil &&
		h.keysRenameDialog.MenuBindingLabel(keymap.ActionFileRenameOpenEncoding) != ""
}

func (h *Handler) closeRenameToolPhase() {
	d := &h.model.FileDialog
	d.RenamePhase = dialog.RenamePhaseMain
	d.FocusedField = 0
}

func (h *Handler) applyRenameToolAndReturnMain() {
	d := &h.model.FileDialog
	if len(d.Fields) < 1 {
		h.closeRenameToolPhase()
		return
	}
	f := &d.Fields[0]
	switch d.RenamePhase {
	case dialog.RenamePhaseSanitize:
		f.Value = dialog.ApplyRenameSanitize(f.Value, d.RenameSanitizeDots, d.RenameSanitizeUnderscores)
	case dialog.RenamePhaseSlugify:
		f.Value = dialog.ApplyRenameSlugify(f.Value, d.RenameSlugifySep)
	case dialog.RenamePhaseEncoding:
		idx := d.RenameEncodingSelected
		if idx < 0 || idx >= len(d.RenameEncodingCandidates) {
			h.closeRenameToolPhase()
			return
		}
		f.Value = d.RenameEncodingCandidates[idx].UTF8
	}
	f.Cursor = len([]rune(f.Value))
	f.PrefillPending = false
	d.RenamePhase = dialog.RenamePhaseMain
	d.FocusedField = 0
}

func (h *Handler) selectRenameEncodingAtFocus() {
	d := &h.model.FileDialog
	if d.FocusedField < 0 || d.FocusedField >= len(d.RenameEncodingCandidates) {
		return
	}
	d.RenameEncodingSelected = d.FocusedField
}
