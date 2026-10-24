package app

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	sftpb "github.com/paranoidi/paras-commander/internal/fsbackend/sftp"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// dialogButtons is what OK / Cancel do in the top-most open dialog.
type dialogButtons struct {
	OK, Cancel func()
	Esc        func() // nil = Cancel; set only where Esc deliberately differs from Alt+C
}

func (b dialogButtons) none() bool { return b.OK == nil && b.Cancel == nil && b.Esc == nil }

// dispatchDialogButton runs the ui.input.dialog-ok / dialog-cancel chord in event against
// the top-most dialog. It is the only place those chords are handled: dialog key handlers
// never look for Alt+O / Alt+C / Esc / F9 themselves. Reports whether the key was consumed.
// Dialogs that expose no buttons (views, menus, quick filter) are left to their own handlers.
func (a *App) dispatchDialogButton(event *tcell.EventKey) bool {
	if a.keys.DialogInput == nil {
		return false
	}
	id, hit := a.keys.DialogInput.Lookup(event)
	if !hit && event.Key() == tcell.KeyRune && unicode.IsUpper(event.Rune()) {
		// Alt+Shift+O / Alt+Shift+C (caps lock) press OK / Cancel like their lowercase chord.
		id, hit = a.keys.DialogInput.Lookup(tcell.NewEventKey(tcell.KeyRune, unicode.ToLower(event.Rune()), event.Modifiers()))
	}
	if !hit || (id != keymap.ActionDialogOK && id != keymap.ActionDialogCancel) {
		return false
	}
	b := a.activeDialogButtons(event)
	if b.none() {
		return false
	}
	fn := b.OK
	if id == keymap.ActionDialogCancel {
		fn = b.Cancel
		if event.Key() == tcell.KeyEsc && b.Esc != nil {
			fn = b.Esc
		}
	}
	if fn != nil {
		fn()
	}
	a.render()
	return true
}

// activeDialogButtons returns the top-most open dialog's buttons (zero value when none
// applies). It switches on a.inputMode() in the same order as inputModeKeyHandlers. A nil
// OK or Cancel means the dialog has no such button and the chord is swallowed. Layered
// dialogs (sub-confirms, sub-phases, completion dropdowns) resolve to their innermost layer.
func (a *App) activeDialogButtons(event *tcell.EventKey) dialogButtons {
	switch a.inputMode() {
	case InputModeElevateDialog:
		return dialogButtons{
			OK:     func() { a.finishElevate(ops.PermissionElevate) },
			Cancel: func() { a.finishElevate(ops.PermissionAbort) },
		}
	case InputModeGroupSelect:
		return dialogButtons{OK: a.confirmGroupSelectFromInput, Cancel: a.closeGroupSelect}
	case InputModeCommandOutputDialog:
		ok, cancel, esc := a.commandsCtrl.OutputDialogButtons()
		return dialogButtons{OK: ok, Cancel: cancel, Esc: esc}
	case InputModeMessageDialog:
		return dialogButtons{OK: a.closeMessageDialog, Cancel: a.closeMessageDialog}
	case InputModePathPicker:
		return dialogButtons{OK: a.dialogCtrl.PathPickerOK, Cancel: a.dialogCtrl.PathPickerCancel}
	case InputModeHistoryDialog:
		return dialogButtons{OK: a.activateHistorySelection, Cancel: a.closeHistoryDialog}
	case InputModePinDialog:
		return dialogButtons{OK: a.pinCtrl.ActivateSelection, Cancel: a.pinCtrl.CloseDialog}
	case InputModeSFTPConnectDialog:
		return dialogButtons{OK: a.executeSFTPConnectDialog, Cancel: a.closeSFTPConnectDialog}
	case InputModeFilterDialog:
		return dialogButtons{OK: a.dialogCtrl.FilterDialogOK, Cancel: a.dialogCtrl.CloseFilterDialog}
	case InputModeFindDialog:
		return dialogButtons{OK: a.findCtrl.ActivateDialogOK, Cancel: a.findCtrl.CloseDialog}
	case InputModeMetaDialog:
		// F9 edits the config from the meta dialog.
		if event.Key() == tcell.KeyF9 {
			return dialogButtons{}
		}
		return dialogButtons{OK: a.metaCtrl.ActivateSelection, Cancel: a.metaCtrl.CloseDialog}
	case InputModeHelpView:
		return dialogButtons{OK: a.closeHelpDialog, Cancel: a.closeHelpDialog}
	case InputModeThemeDialog:
		return dialogButtons{OK: a.activateThemeDialogSelection, Cancel: a.cancelThemeDialog}
	case InputModeSortDialog:
		return dialogButtons{OK: a.applySortDialog, Cancel: a.closeSortDialog}
	case InputModeListingFormatDialog:
		return dialogButtons{OK: a.applyListingFormatDialog, Cancel: a.closeListingFormatDialog}
	case InputModeConfigDialog:
		return a.configDialogButtons(event)
	case InputModePreviewSettingsDialog:
		return dialogButtons{OK: a.applyPreviewSettingsDialog, Cancel: a.closePreviewSettingsDialog}
	case InputModeDebounceCalibrateDialog:
		if a.model.DebounceCalibrateDialog.Phase == dialog.DebounceCalibrateMeasuring {
			return dialogButtons{Cancel: a.abortDebounceCalibrateMeasuring}
		}
		return dialogButtons{OK: a.applyDebounceCalibrateDialog, Cancel: a.closeDebounceCalibrateDialog}
	case InputModeHostKeyDialog:
		return dialogButtons{Cancel: func() { a.finishHostKeyDialog(sftpb.HostKeyReject) }}
	case InputModeFileDialog:
		return dialogButtons{OK: a.dialogCtrl.FileDialogOK, Cancel: a.dialogCtrl.FileDialogCancel}
	case InputModeDialog:
		return a.modalDialogButtons(event)
	case InputModeDedupProgressDialog:
		b := dialogButtons{Cancel: a.dedupCtrl.Close}
		if a.model.DedupSnapshot.Phase == comparepkg.DedupAwaitConfirm {
			b.OK = a.dedupCtrl.Confirm
		}
		return b
	case InputModeDedupReturnDialog:
		return dialogButtons{Cancel: a.dedupCtrl.ReturnDialogCancel}
	case InputModeCompareView:
		switch {
		case a.model.CompareFilterDialog.Open:
			return dialogButtons{OK: a.compareCtrl.FilterDialogOK, Cancel: a.compareCtrl.FilterDialogCancel}
		case a.model.CompareMergeDialog.Open:
			return dialogButtons{OK: a.compareCtrl.MergeDialogOK, Cancel: a.compareCtrl.MergeDialogCancel}
		}
	}
	return dialogButtons{}
}

// configDialogButtons resolves the Configuration dialog: an open Yes/No sub-confirm takes
// OK / Cancel first, and F9 stays "edit config.toml".
func (a *App) configDialogButtons(event *tcell.EventKey) dialogButtons {
	st := &a.model.ConfigDialog
	switch {
	case st.EditStubConfirm:
		return dialogButtons{
			OK:     func() { st.EditStubConfirm = false; a.createAndEditConfigStub() },
			Cancel: func() { st.EditStubConfirm = false },
		}
	case st.ResetDefaultsConfirm:
		return dialogButtons{
			OK:     func() { st.ResetDefaultsConfirm = false; a.resetConfigToDefaults() },
			Cancel: func() { st.ResetDefaultsConfirm = false },
		}
	case event.Key() == tcell.KeyF9:
		return dialogButtons{}
	}
	return dialogButtons{OK: a.applyConfigDialog, Cancel: a.closeConfigDialog}
}

// modalDialogButtons resolves the dialogs that share InputModeDialog (see handleDialogKey).
func (a *App) modalDialogButtons(event *tcell.EventKey) dialogButtons {
	switch {
	case a.model.ConflictDialog.Open:
		// Alt+letter picks a conflict choice (Alt+O = Overwrite, Alt+C = Cancel), so only the
		// non-letter chords (M-Enter, Esc, F9) map to OK / postpone.
		if !a.model.ConflictDialog.Advanced && event.Key() == tcell.KeyRune {
			return dialogButtons{}
		}
		return dialogButtons{OK: a.jobsCtrl.DialogOK, Cancel: a.jobsCtrl.DialogCancel}
	case a.model.TransferDialog.Open:
		return dialogButtons{OK: a.dialogCtrl.TransferDialogOK, Cancel: a.dialogCtrl.TransferDialogCancel}
	case a.model.FlattenDialog.Open:
		return dialogButtons{OK: a.dialogCtrl.FlattenDialogOK, Cancel: a.dialogCtrl.FlattenDialogCancel}
	case a.model.QuitConfirm.Open:
		return dialogButtons{Cancel: func() { a.model.QuitConfirm = dialog.QuitConfirmState{} }}
	case a.model.DedupEmptyDirsConfirm.Open:
		// Cancel backs out: nothing is deleted and the dedup marks stay.
		d := &a.model.DedupEmptyDirsConfirm
		return dialogButtons{
			OK: func() {
				focus := d.Focus
				a.model.DedupEmptyDirsConfirm = dialog.DedupEmptyDirsConfirmState{}
				if focus < 2 {
					a.dedupCtrl.DeleteMarked(focus == 0)
				}
			},
			Cancel: func() { a.model.DedupEmptyDirsConfirm = dialog.DedupEmptyDirsConfirmState{} },
		}
	case a.model.UndoConfirm.Open:
		return dialogButtons{
			OK:     func() { a.answerUndoConfirm(a.model.UndoConfirm.Focus == dialog.UndoYes) },
			Cancel: func() { a.answerUndoConfirm(false) },
		}
	case a.model.StashRestoreDialog.Open:
		return dialogButtons{
			OK:     func() { a.applyStashRestoreChoice(stashRestoreFocusAction(a.model.StashRestoreDialog.Focus)) },
			Cancel: a.closeStashRestoreDialog,
		}
	}
	return dialogButtons{}
}
