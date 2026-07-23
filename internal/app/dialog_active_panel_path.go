package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// destinationShortcutFooterKeys builds the "Active path"/"Inactive path" footer hints shared by
// every dialog overlay that binds ActionDestinationActivePanel/ActionDestinationInactivePanel.
func destinationShortcutFooterKeys(keys *keymap.Map, eligible bool) []menu.FunctionKey {
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

// tryDestinationShortcut applies applyActive/applyInactive when ev matches a chord bound to
// ActionDestinationActivePanel/ActionDestinationInactivePanel in keys, gated by eligible.
func tryDestinationShortcut(ev *tcell.EventKey, keys *keymap.Map, eligible bool, applyActive, applyInactive func()) bool {
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

func (a *App) flattenDialogDestinationFooterEligible() bool {
	return a.model.FlattenDialog.Open && a.model.FlattenDialog.FocusField == 0
}

func flattenDialogOverlayFooterKeys(a *App, keys *keymap.Map) []menu.FunctionKey {
	return destinationShortcutFooterKeys(keys, a.flattenDialogDestinationFooterEligible())
}

// tryFlattenDialogDestinationShortcut sets the flatten destination to the active or
// inactive panel path when the user presses a chord from [dialog.flatten]
// while the destination row is focused.
func (a *App) tryFlattenDialogDestinationShortcut(ev *tcell.EventKey) bool {
	return tryDestinationShortcut(ev, a.keysFlattenDialog, a.flattenDialogDestinationFooterEligible(),
		a.applyFlattenDestinationFromActivePanelState, a.applyFlattenDestinationFromInactivePanel)
}

func (a *App) applyFlattenDestinationFromActivePanelState() {
	d := &a.model.FlattenDialog
	d.Destination = transferPrefilledDestination(a.activePanel().PathString())
	d.DestSubFocus = dialog.FlattenDestSubFocusText
	a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
	a.armFlattenDestinationValidateTimer()
}

func (a *App) applyFlattenDestinationFromInactivePanel() {
	d := &a.model.FlattenDialog
	d.Destination = transferPrefilledDestination(a.inactivePanel().PathString())
	d.DestSubFocus = dialog.FlattenDestSubFocusText
	a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
	a.armFlattenDestinationValidateTimer()
}

func (a *App) transferDialogDestinationFooterEligible() bool {
	d := a.model.TransferDialog
	return d.Open && d.Phase == dialog.TransferPhaseDestination && d.FocusField == 0
}

func transferDialogOverlayFooterKeys(a *App, keys *keymap.Map) []menu.FunctionKey {
	return destinationShortcutFooterKeys(keys, a.transferDialogDestinationFooterEligible())
}

// tryTransferDialogDestinationShortcut sets the transfer (copy/move) destination to the active
// or inactive panel path when the user presses a chord from [dialog.transfer]
// while the destination row is focused.
func (a *App) tryTransferDialogDestinationShortcut(ev *tcell.EventKey) bool {
	return tryDestinationShortcut(ev, a.keysTransferDialog, a.transferDialogDestinationFooterEligible(),
		a.applyTransferDestinationFromActivePanel, a.applyTransferDestinationFromInactivePanel)
}

func (a *App) applyTransferDestinationFromActivePanel() {
	d := &a.model.TransferDialog
	d.Destination = transferPrefilledDestination(a.activePanel().PathString())
	d.DestSubFocus = dialog.TransferDestSubFocusText
	a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
	a.armTransferDestinationValidateTimer()
}

func (a *App) applyTransferDestinationFromInactivePanel() {
	d := &a.model.TransferDialog
	d.Destination = transferPrefilledDestination(a.inactivePanel().PathString())
	d.DestSubFocus = dialog.TransferDestSubFocusText
	a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
	a.armTransferDestinationValidateTimer()
}
