package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// DestinationShortcutFooterKeys builds the "Active path"/"Inactive path" footer hints shared by
// every dialog overlay that binds ActionDestinationActivePanel/ActionDestinationInactivePanel.
// Shared by the transfer dialog here and the flatten dialog (internal/app).
func DestinationShortcutFooterKeys(keys *keymap.Map, eligible bool) []menu.FunctionKey {
	if keys == nil || !eligible {
		return nil
	}
	var out []menu.FunctionKey
	if lbl := keys.MenuBindingLabel(keymap.ActionDestinationActivePanel); lbl != "" {
		out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Active path ◄"})
	}
	if lbl := keys.MenuBindingLabel(keymap.ActionDestinationInactivePanel); lbl != "" {
		out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Inactive path ►"})
	}
	return out
}

// TryDestinationShortcut applies applyActive/applyInactive when ev matches a chord bound to
// ActionDestinationActivePanel/ActionDestinationInactivePanel in keys, gated by eligible.
// Shared by the transfer dialog here and the flatten dialog (internal/app).
func TryDestinationShortcut(ev *tcell.EventKey, keys *keymap.Map, eligible bool, applyActive, applyInactive func()) bool {
	if keys == nil || !eligible {
		return false
	}
	id, ok := keys.Lookup(ev)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionDestinationActivePanel:
		applyActive()
		return true
	case keymap.ActionDestinationInactivePanel:
		applyInactive()
		return true
	default:
		return false
	}
}

func (h *Handler) transferDialogDestinationFooterEligible() bool {
	d := h.model.TransferDialog
	return d.Open && d.Phase == dialog.TransferPhaseDestination && d.FocusField == 0
}

// TransferDialogOverlayFooterKeys builds the transfer dialog's "Active path"/"Inactive path"
// footer hints when its destination field is focused.
func (h *Handler) TransferDialogOverlayFooterKeys(keys *keymap.Map) []menu.FunctionKey {
	return DestinationShortcutFooterKeys(keys, h.transferDialogDestinationFooterEligible())
}

// TryTransferDialogDestinationShortcut sets the transfer (copy/move) destination to the active
// or inactive panel path when the user presses a chord from [dialog.transfer]
// while the destination row is focused.
func (h *Handler) TryTransferDialogDestinationShortcut(ev *tcell.EventKey) bool {
	return TryDestinationShortcut(ev, h.keysTransferDialog, h.transferDialogDestinationFooterEligible(),
		h.applyTransferDestinationFromActivePanel, h.applyTransferDestinationFromInactivePanel)
}

func (h *Handler) applyTransferDestinationFromActivePanel() {
	d := &h.model.TransferDialog
	d.Destination = TransferPrefilledDestination(h.host.ActivePanel().PathString())
	d.DestSubFocus = dialog.TransferDestSubFocusText
	h.SyncPathFieldCompletion(&d.Destination, h.TransferDestinationTextWidth())
	h.ArmTransferDestinationValidateTimer()
}

func (h *Handler) applyTransferDestinationFromInactivePanel() {
	d := &h.model.TransferDialog
	d.Destination = TransferPrefilledDestination(h.host.InactivePanel().PathString())
	d.DestSubFocus = dialog.TransferDestSubFocusText
	h.SyncPathFieldCompletion(&d.Destination, h.TransferDestinationTextWidth())
	h.ArmTransferDestinationValidateTimer()
}

func (h *Handler) flattenDialogDestinationFooterEligible() bool {
	return h.model.FlattenDialog.Open && h.model.FlattenDialog.FocusField == 0
}

// FlattenDialogOverlayFooterKeys builds the flatten dialog's "Active path"/"Inactive path"
// footer hints when its destination field is focused.
func (h *Handler) FlattenDialogOverlayFooterKeys(keys *keymap.Map) []menu.FunctionKey {
	return DestinationShortcutFooterKeys(keys, h.flattenDialogDestinationFooterEligible())
}

// TryFlattenDialogDestinationShortcut sets the flatten destination to the active or inactive
// panel path when the user presses a chord from [dialog.flatten] while the destination row is
// focused.
func (h *Handler) TryFlattenDialogDestinationShortcut(ev *tcell.EventKey) bool {
	return TryDestinationShortcut(ev, h.keysFlattenDialog, h.flattenDialogDestinationFooterEligible(),
		h.applyFlattenDestinationFromActivePanel, h.applyFlattenDestinationFromInactivePanel)
}

func (h *Handler) applyFlattenDestinationFromActivePanel() {
	d := &h.model.FlattenDialog
	d.Destination = TransferPrefilledDestination(h.host.ActivePanel().PathString())
	d.DestSubFocus = dialog.FlattenDestSubFocusText
	h.SyncPathFieldCompletion(&d.Destination, h.TransferDestinationTextWidth())
	h.ArmFlattenDestinationValidateTimer()
}

func (h *Handler) applyFlattenDestinationFromInactivePanel() {
	d := &h.model.FlattenDialog
	d.Destination = TransferPrefilledDestination(h.host.InactivePanel().PathString())
	d.DestSubFocus = dialog.FlattenDestSubFocusText
	h.SyncPathFieldCompletion(&d.Destination, h.TransferDestinationTextWidth())
	h.ArmFlattenDestinationValidateTimer()
}
