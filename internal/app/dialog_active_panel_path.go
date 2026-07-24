package app

import (
	"github.com/gdamore/tcell/v2"
	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func (a *App) flattenDialogDestinationFooterEligible() bool {
	return a.model.FlattenDialog.Open && a.model.FlattenDialog.FocusField == 0
}

func flattenDialogOverlayFooterKeys(a *App, keys *keymap.Map) []menu.FunctionKey {
	return dialogctrl.DestinationShortcutFooterKeys(keys, a.flattenDialogDestinationFooterEligible())
}

// tryFlattenDialogDestinationShortcut sets the flatten destination to the active or
// inactive panel path when the user presses a chord from [dialog.flatten]
// while the destination row is focused.
func (a *App) tryFlattenDialogDestinationShortcut(ev *tcell.EventKey) bool {
	return dialogctrl.TryDestinationShortcut(ev, a.keys.FlattenDialog, a.flattenDialogDestinationFooterEligible(),
		a.applyFlattenDestinationFromActivePanelState, a.applyFlattenDestinationFromInactivePanel)
}

func (a *App) applyFlattenDestinationFromActivePanelState() {
	d := &a.model.FlattenDialog
	d.Destination = dialogctrl.TransferPrefilledDestination(a.activePanel().PathString())
	d.DestSubFocus = dialog.FlattenDestSubFocusText
	a.dialogCtrl.SyncPathFieldCompletion(&d.Destination, a.dialogCtrl.TransferDestinationTextWidth())
	a.dialogCtrl.ArmFlattenDestinationValidateTimer()
}

func (a *App) applyFlattenDestinationFromInactivePanel() {
	d := &a.model.FlattenDialog
	d.Destination = dialogctrl.TransferPrefilledDestination(a.inactivePanel().PathString())
	d.DestSubFocus = dialog.FlattenDestSubFocusText
	a.dialogCtrl.SyncPathFieldCompletion(&d.Destination, a.dialogCtrl.TransferDestinationTextWidth())
	a.dialogCtrl.ArmFlattenDestinationValidateTimer()
}
