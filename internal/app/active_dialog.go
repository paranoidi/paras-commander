package app

import (
	"time"

	"github.com/gdamore/tcell/v2"
	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	sftpb "github.com/paranoidi/paras-commander/internal/fsbackend/sftp"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// activeDialog is the top-most open dlg dialog: its spec as drawn (built on demand; ok is false
// when it does not fit the screen), the focus field its key handler reads, the function its key
// handler applies dlg events with (reporting whether the app should quit), and what the central
// OK / Cancel chords do in it.
type activeDialog struct {
	spec    func() (dlg.Spec, bool)
	focus   *dlg.ID
	apply   func(dlg.Event) (quit bool)
	buttons dialogButtons
}

// dialogButtons is what OK / Cancel do in the top-most open dialog.
type dialogButtons struct {
	OK, Cancel func()
	Esc        func() // nil = Cancel; set only where Esc deliberately differs from Alt+C
}

func (b dialogButtons) none() bool { return b.OK == nil && b.Cancel == nil && b.Esc == nil }

// noQuit adapts an event function that never quits to activeDialog.apply.
func noQuit(f func(dlg.Event)) func(dlg.Event) bool {
	return func(ev dlg.Event) bool { f(ev); return false }
}

// fits adapts a spec builder whose dialog always draws to activeDialog.spec.
func fits(view func() dlg.Spec) func() (dlg.Spec, bool) {
	return func() (dlg.Spec, bool) { return view(), true }
}

// activeDialog returns the top-most open dlg dialog, the one table the button chords
// (dispatchDialogButton), the mouse (handleDialogMouse) and the dialog invariants read. It switches
// on a.inputMode() and every dlg dialog adds one case. spec must be the one the renderer draws
// (same builder, same layout inputs) and the one the dialog's key handler passes to dlg.HandleKey,
// and apply the event function that handler calls after it. Dialogs whose key handler keeps its
// own focus (message, pin) get a scratch focus. event is the key the buttons answer (F9 and the
// conflict screen's Alt+letter choices take no button); the mouse passes Enter.
func (a *App) activeDialog(event *tcell.EventKey) (activeDialog, bool) {
	found := func(spec func() (dlg.Spec, bool), focus *dlg.ID, apply func(dlg.Event) bool, b dialogButtons) (activeDialog, bool) {
		return activeDialog{spec, focus, apply, b}, true
	}
	scratch := func(id dlg.ID) *dlg.ID { return &id }
	layout := func() ui.Layout {
		w, h := a.screen.Size()
		return a.layoutForTerminalSize(w, h)
	}
	m := &a.model
	switch a.inputMode() {
	case InputModeElevateDialog:
		return found(fits(func() dlg.Spec { return dialog.ElevateView(m.ElevateDialog) }), &m.ElevateDialog.Focus, noQuit(a.applyElevateEvent), dialogButtons{
			OK:     func() { a.finishElevate(ops.PermissionElevate) },
			Cancel: func() { a.finishElevate(ops.PermissionAbort) },
		})
	case InputModeGroupSelect:
		return found(fits(a.groupSelectView), &m.GroupSelect.Focus, noQuit(func(ev dlg.Event) {
			dialogctrl.ApplyPatternDialogEvent(a.groupSelectPatternKeys(), ev)
		}), dialogButtons{OK: a.confirmGroupSelectFromInput, Cancel: a.closeGroupSelect})
	case InputModeCommandOutputDialog:
		ok, cancel, esc := a.commandsCtrl.OutputDialogButtons()
		return found(func() (dlg.Spec, bool) {
			spec, _, fit := dialog.CommandOutputDialogView(layout(), m.CommandOutputDialog, dialog.CommandOutputViewContext{Styles: a.styles})
			return spec, fit
		}, &m.CommandOutputDialog.Focus, noQuit(a.commandsCtrl.ApplyOutputDialogEvent), dialogButtons{OK: ok, Cancel: cancel, Esc: esc})
	case InputModeMessageDialog:
		return found(fits(func() dlg.Spec { return dialog.MessageDialogView(m.MessageDialog, layout()) }), scratch(dlg.OK), noQuit(a.applyMessageDialogEvent),
			dialogButtons{OK: a.closeMessageDialog, Cancel: a.closeMessageDialog})
	case InputModePathPicker:
		return found(fits(a.dialogCtrl.PathPickerView), &m.PathPicker.Focus, noQuit(a.dialogCtrl.ApplyPathPickerEvent),
			dialogButtons{OK: a.dialogCtrl.PathPickerOK, Cancel: a.dialogCtrl.PathPickerCancel})
	case InputModeHistoryDialog:
		return found(fits(a.historyView), &m.HistoryDialog.Focus, noQuit(a.applyHistoryEvent),
			dialogButtons{OK: a.activateHistorySelection, Cancel: a.closeHistoryDialog})
	case InputModePinDialog:
		return found(fits(a.pinCtrl.DialogView), scratch(dialog.PinFilter), noQuit(a.pinCtrl.ApplyDialogEvent),
			dialogButtons{OK: a.pinCtrl.ActivateSelection, Cancel: a.pinCtrl.CloseDialog})
	case InputModeSFTPConnectDialog:
		return found(fits(a.sftpConnectView), &m.SFTPConnectDialog.Focus, noQuit(a.applySFTPConnectEvent),
			dialogButtons{OK: a.executeSFTPConnectDialog, Cancel: a.closeSFTPConnectDialog})
	case InputModeFilterDialog:
		return found(fits(a.dialogCtrl.FilterDialogView), &m.FilterDialog.Focus, noQuit(a.dialogCtrl.ApplyFilterDialogEvent),
			dialogButtons{OK: a.dialogCtrl.FilterDialogOK, Cancel: a.dialogCtrl.CloseFilterDialog})
	case InputModeFindDialog:
		return found(fits(a.findCtrl.DialogSpec), &m.FindDialog.Focus, noQuit(a.findCtrl.ApplyDialogEvent),
			dialogButtons{OK: a.findCtrl.ActivateDialogOK, Cancel: a.findCtrl.CloseDialog})
	case InputModeMetaDialog:
		b := dialogButtons{OK: a.metaCtrl.ActivateSelection, Cancel: a.metaCtrl.CloseDialog}
		if event.Key() == tcell.KeyF9 {
			b = dialogButtons{} // F9 edits the config from the meta dialog
		}
		return found(fits(func() dlg.Spec { return dialog.MetaDialogView(m.MetaDialog) }), &m.MetaDialog.Focus, noQuit(a.metaCtrl.ApplyDialogEvent), b)
	case InputModeHelpView:
		return found(func() (dlg.Spec, bool) {
			spec, _, fit := dialog.HelpDialogView(m.HelpView, layout(), a.styles)
			return spec, fit
		}, &m.HelpView.Focus, a.applyHelpEvent, dialogButtons{OK: a.closeHelpDialog, Cancel: a.closeHelpDialog})
	case InputModeThemeDialog:
		return found(fits(a.themeDialogView), &m.ThemeDialog.Focus, noQuit(a.applyThemeDialogEvent),
			dialogButtons{OK: a.activateThemeDialogSelection, Cancel: a.cancelThemeDialog})
	case InputModeSortDialog:
		return found(fits(func() dlg.Spec { return dialog.SortDialogView(m.SortDialog) }), &m.SortDialog.Focus, noQuit(a.applySortDialogEvent),
			dialogButtons{OK: a.applySortDialog, Cancel: a.closeSortDialog})
	case InputModeStatusCommandsDialog:
		return found(fits(func() dlg.Spec { return dialog.StatusCommandsDialogView(m.StatusCommandsDialog) }), &m.StatusCommandsDialog.Focus, noQuit(a.applyStatusCommandsDialogEvent),
			dialogButtons{OK: a.applyStatusCommandsDialog, Cancel: a.closeStatusCommandsDialog})
	case InputModeListingFormatDialog:
		return found(fits(func() dlg.Spec { return dialog.ListingFormatView(m.ListingFormatDialog) }), &m.ListingFormatDialog.Focus, noQuit(a.applyListingFormatEvent),
			dialogButtons{OK: a.applyListingFormatDialog, Cancel: a.closeListingFormatDialog})
	case InputModeConfigDialog:
		return a.configActiveDialog(event)
	case InputModePreviewSettingsDialog:
		return found(fits(a.previewSettingsView), &m.PreviewSettingsDialog.Focus, noQuit(a.applyPreviewSettingsEvent),
			dialogButtons{OK: a.applyPreviewSettingsDialog, Cancel: a.closePreviewSettingsDialog})
	case InputModeDebounceCalibrateDialog:
		b := dialogButtons{OK: a.applyDebounceCalibrateDialog, Cancel: a.closeDebounceCalibrateDialog}
		if m.DebounceCalibrateDialog.Phase == dialog.DebounceCalibrateMeasuring {
			b = dialogButtons{Cancel: a.abortDebounceCalibrateMeasuring}
		}
		return found(fits(func() dlg.Spec { return dialog.DebounceCalibrateView(m.DebounceCalibrateDialog) }), &m.DebounceCalibrateDialog.Focus, noQuit(a.applyDebounceCalibrateEvent), b)
	case InputModeHostKeyDialog:
		return found(fits(func() dlg.Spec { return dialog.HostKeyView(m.HostKeyDialog) }), &m.HostKeyDialog.Focus, noQuit(a.applyHostKeyEvent),
			dialogButtons{Cancel: func() { a.finishHostKeyDialog(sftpb.HostKeyReject) }})
	case InputModeFileDialog:
		s := a.dialogCtrl.FileDialogScreen()
		return found(fits(func() dlg.Spec { return s.Spec }), s.Focus, noQuit(a.dialogCtrl.ApplyFileDialogEvent),
			dialogButtons{OK: a.dialogCtrl.FileDialogOK, Cancel: a.dialogCtrl.FileDialogCancel})
	case InputModeDialog:
		return a.modalActiveDialog(event)
	case InputModeDedupProgressDialog:
		b := dialogButtons{Cancel: a.dedupCtrl.Close}
		if m.DedupSnapshot.Phase == comparepkg.DedupAwaitConfirm {
			b.OK = a.dedupCtrl.Confirm
		}
		return found(fits(func() dlg.Spec { return dialog.DedupProgressView(m.DedupSnapshot, m.UserHomeDir, time.Now()) }), &m.DedupProgressDialog.Focus, noQuit(a.dedupCtrl.ApplyProgressDialogEvent), b)
	case InputModeDedupReturnDialog:
		return found(fits(func() dlg.Spec { return ui.DedupReturnSpec(m) }), &m.DedupReturnDialog.Focus, noQuit(a.dedupCtrl.ApplyReturnDialogEvent),
			dialogButtons{Cancel: a.dedupCtrl.ReturnDialogCancel})
	case InputModeJobsView:
		if m.JobsResultFilterDialog.Open {
			return found(fits(func() dlg.Spec { return dialog.JobsResultFilterView(m.JobsResultFilterDialog) }), &m.JobsResultFilterDialog.Focus, noQuit(a.jobsCtrl.ApplyResultFilterDialogEvent),
				dialogButtons{OK: a.jobsCtrl.ResultFilterDialogOK, Cancel: a.jobsCtrl.ResultFilterDialogCancel})
		}
	case InputModeCompareView:
		switch {
		case m.CompareFilterDialog.Open:
			return found(fits(func() dlg.Spec { return dialog.CompareFilterView(m.CompareFilterDialog) }), &m.CompareFilterDialog.Focus, noQuit(a.compareCtrl.ApplyFilterDialogEvent),
				dialogButtons{OK: a.compareCtrl.FilterDialogOK, Cancel: a.compareCtrl.FilterDialogCancel})
		case m.CompareMergeDialog.Open:
			return found(fits(func() dlg.Spec { return dialog.CompareMergeView(m.CompareMergeDialog, m.UserHomeDir) }), &m.CompareMergeDialog.Focus, noQuit(a.compareCtrl.ApplyMergeDialogEvent),
				dialogButtons{OK: a.compareCtrl.MergeDialogOK, Cancel: a.compareCtrl.MergeDialogCancel})
		}
	}
	return activeDialog{}, false
}

// configActiveDialog resolves the Configuration dialog: its screen on top (an open Yes/No
// sub-confirm or the dialog), whose OK / Cancel answer the confirm first; F9 stays "edit
// config.toml". applyConfigDialogEvent routes the sub-confirms' events itself.
func (a *App) configActiveDialog(event *tcell.EventKey) (activeDialog, bool) {
	st := &a.model.ConfigDialog
	s := a.configDialogScreen()
	b := dialogButtons{OK: a.applyConfigDialog, Cancel: a.closeConfigDialog}
	switch {
	case st.EditStubConfirm:
		b = dialogButtons{
			OK:     func() { st.EditStubConfirm = false; a.createAndEditConfigStub() },
			Cancel: func() { st.EditStubConfirm = false },
		}
	case st.ResetDefaultsConfirm:
		b = dialogButtons{
			OK:     func() { st.ResetDefaultsConfirm = false; a.resetConfigToDefaults() },
			Cancel: func() { st.ResetDefaultsConfirm = false },
		}
	case event.Key() == tcell.KeyF9:
		b = dialogButtons{}
	}
	return activeDialog{func() (dlg.Spec, bool) { return s.Spec, true }, s.Focus, noQuit(a.applyConfigDialogEvent), b}, true
}

// modalActiveDialog resolves the dialogs that share InputModeDialog, in handleDialogKey's order.
func (a *App) modalActiveDialog(event *tcell.EventKey) (activeDialog, bool) {
	m := &a.model
	found := func(view func() dlg.Spec, focus *dlg.ID, apply func(dlg.Event) bool, b dialogButtons) (activeDialog, bool) {
		return activeDialog{fits(view), focus, apply, b}, true
	}
	switch {
	case m.ConflictDialog.Open:
		b := dialogButtons{OK: a.jobsCtrl.DialogOK, Cancel: a.jobsCtrl.DialogCancel}
		// Alt+letter picks a conflict choice (Alt+O = Overwrite, Alt+C = Cancel), so only the
		// non-letter chords (M-Enter, Esc, F9) map to OK / postpone.
		if !m.ConflictDialog.Advanced && event.Key() == tcell.KeyRune {
			b = dialogButtons{}
		}
		s := a.jobsCtrl.BlockerDialogScreen()
		return found(func() dlg.Spec { return s.Spec }, s.Focus, noQuit(a.jobsCtrl.ApplyBlockerDialogEvent), b)
	case m.TransferDialog.Open:
		s := a.dialogCtrl.TransferDialogScreen()
		return found(func() dlg.Spec { return s.Spec }, s.Focus, noQuit(a.dialogCtrl.ApplyTransferEvent),
			dialogButtons{OK: a.dialogCtrl.TransferDialogOK, Cancel: a.dialogCtrl.TransferDialogCancel})
	case m.FlattenDialog.Open:
		return found(func() dlg.Spec { return dialog.FlattenView(m.FlattenDialog) }, &m.FlattenDialog.Focus, noQuit(a.dialogCtrl.ApplyFlattenEvent),
			dialogButtons{OK: a.dialogCtrl.FlattenDialogOK, Cancel: a.dialogCtrl.FlattenDialogCancel})
	case m.QuitConfirm.Open:
		return found(func() dlg.Spec { return dialog.QuitConfirmView(m.QuitConfirm) }, &m.QuitConfirm.Focus, a.applyQuitConfirmEvent,
			dialogButtons{Cancel: func() { m.QuitConfirm = dialog.QuitConfirmState{} }})
	case m.DedupEmptyDirsConfirm.Open:
		// Cancel backs out: nothing is deleted and the dedup marks stay.
		return found(a.dedupEmptyDirsConfirmView, &m.DedupEmptyDirsConfirm.Focus, noQuit(a.applyDedupEmptyDirsConfirmEvent), dialogButtons{
			OK:     func() { a.answerDedupEmptyDirsConfirm(m.DedupEmptyDirsConfirm.Focus) },
			Cancel: func() { m.DedupEmptyDirsConfirm = dialog.DedupEmptyDirsConfirmState{} },
		})
	case m.UndoConfirm.Open:
		return found(func() dlg.Spec { return dialog.UndoConfirmView(m.UndoConfirm) }, &m.UndoConfirm.Focus, noQuit(a.applyUndoConfirmEvent), dialogButtons{
			OK:     func() { a.answerUndoConfirm(m.UndoConfirm.Focus == dialog.UndoYes) },
			Cancel: func() { a.answerUndoConfirm(false) },
		})
	case m.StashRestoreDialog.Open:
		return found(func() dlg.Spec { return dialog.StashRestoreView(m.StashRestoreDialog) }, &m.StashRestoreDialog.Focus, noQuit(a.applyStashRestoreEvent), dialogButtons{
			OK:     func() { a.applyStashRestoreChoice(stashRestoreFocusAction(m.StashRestoreDialog.Focus)) },
			Cancel: a.closeStashRestoreDialog,
		})
	case m.CompareSelection.Open:
		return found(func() dlg.Spec { return dialog.CompareSelectionView(m.CompareSelection) }, &m.CompareSelection.Focus, noQuit(a.applyCompareSelectionEvent), dialogButtons{
			OK:     func() { a.answerCompareSelection(m.CompareSelection.Focus) },
			Cancel: func() { a.answerCompareSelection(dlg.Cancel) },
		})
	}
	return activeDialog{}, false
}
