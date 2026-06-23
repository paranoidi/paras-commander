package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func (a *App) flattenDialogDestinationFooterEligible() bool {
	return a.model.FlattenDialog.Open && a.model.FlattenDialog.FocusField == 0
}

func flattenDialogOverlayFooterKeys(a *App, keys *keymap.Map) []menu.FunctionKey {
	if keys == nil || !a.flattenDialogDestinationFooterEligible() {
		return nil
	}
	var out []menu.FunctionKey
	if lbl := keys.MenuBindingLabel(keymap.ActionFlattenDestinationActive); lbl != "" {
		out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Active path"})
	}
	if lbl := keys.MenuBindingLabel(keymap.ActionFlattenDestinationInactive); lbl != "" {
		out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Inactive path"})
	}
	return out
}

// tryFlattenDialogDestinationShortcut sets the flatten destination to the active or
// inactive panel path when the user presses a chord from [dialog.flatten]
// while the destination row is focused.
func (a *App) tryFlattenDialogDestinationShortcut(ev *tcell.EventKey) bool {
	if a.keysFlattenDialog == nil || !a.flattenDialogDestinationFooterEligible() {
		return false
	}
	id, ok := a.keysFlattenDialog.Lookup(ev)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionFlattenDestinationActive:
		a.applyFlattenDestinationFromActivePanelState()
		return true
	case keymap.ActionFlattenDestinationInactive:
		a.applyFlattenDestinationFromInactivePanel()
		return true
	default:
		return false
	}
}

func (a *App) applyFlattenDestinationFromActivePanelState() {
	d := &a.model.FlattenDialog
	d.Destination = transferPrefilledDestination(a.activePanel().PathString())
	d.DestSubFocus = ui.FlattenDestSubFocusText
	a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
	a.armFlattenDestinationValidateTimer()
}

func (a *App) applyFlattenDestinationFromInactivePanel() {
	d := &a.model.FlattenDialog
	d.Destination = transferPrefilledDestination(a.inactivePanel().PathString())
	d.DestSubFocus = ui.FlattenDestSubFocusText
	a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
	a.armFlattenDestinationValidateTimer()
}
