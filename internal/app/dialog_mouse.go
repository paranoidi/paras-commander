package app

import (
	"time"

	"github.com/gdamore/tcell/v2"
	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// dialogMouse is the top-most dlg dialog as the mouse sees it: its spec as drawn, the focus field
// its key handler reads, and the function its key handler applies dlg events with (reporting
// whether the app should quit).
type dialogMouse struct {
	spec  dlg.Spec
	focus *dlg.ID
	apply func(dlg.Event) (quit bool)
}

// noQuit adapts an event function that never quits to dialogMouse.apply.
func noQuit(f func(dlg.Event)) func(dlg.Event) bool {
	return func(ev dlg.Event) bool { f(ev); return false }
}

// activeDialogMouse returns the top-most open dlg dialog. It switches on a.inputMode() in the same
// order as activeDialogButtons, and every dlg dialog adds one case. Each spec must be the one the
// renderer draws (same builder, same layout inputs), or clicks land on the wrong widget, and apply
// must be the event function the dialog's key handler calls after dlg.HandleKey. Dialogs whose key
// handler keeps its own focus (message, pin) get a scratch focus.
func (a *App) activeDialogMouse(layout ui.Layout) (dialogMouse, bool) {
	w, _ := a.screen.Size()
	found := func(spec dlg.Spec, focus *dlg.ID, apply func(dlg.Event) bool) (dialogMouse, bool) {
		return dialogMouse{spec, focus, apply}, true
	}
	scratch := func(id dlg.ID) *dlg.ID { return &id }
	m := &a.model
	switch a.inputMode() {
	case InputModeElevateDialog:
		return found(dialog.ElevateView(m.ElevateDialog), &m.ElevateDialog.Focus, noQuit(a.applyElevateEvent))
	case InputModeGroupSelect:
		return found(a.groupSelectView(), &m.GroupSelect.Focus, noQuit(func(ev dlg.Event) {
			dialogctrl.ApplyPatternDialogEvent(a.groupSelectPatternKeys(), ev)
		}))
	case InputModeCommandOutputDialog:
		if spec, _, ok := dialog.CommandOutputDialogView(layout, m.CommandOutputDialog, dialog.CommandOutputViewContext{Styles: a.styles}); ok {
			return found(spec, &m.CommandOutputDialog.Focus, noQuit(a.commandsCtrl.ApplyOutputDialogEvent))
		}
	case InputModeMessageDialog:
		return found(dialog.MessageDialogView(m.MessageDialog, layout), scratch(dlg.OK), noQuit(a.applyMessageDialogEvent))
	case InputModePathPicker:
		return found(a.dialogCtrl.PathPickerView(), &m.PathPicker.Focus, noQuit(a.dialogCtrl.ApplyPathPickerEvent))
	case InputModeHistoryDialog:
		return found(a.historyView(), &m.HistoryDialog.Focus, noQuit(a.applyHistoryEvent))
	case InputModePinDialog:
		return found(a.pinCtrl.DialogView(), scratch(dialog.PinFilter), noQuit(a.pinCtrl.ApplyDialogEvent))
	case InputModeSFTPConnectDialog:
		return found(a.sftpConnectView(), &m.SFTPConnectDialog.Focus, noQuit(a.applySFTPConnectEvent))
	case InputModeFilterDialog:
		return found(a.dialogCtrl.FilterDialogView(), &m.FilterDialog.Focus, noQuit(a.dialogCtrl.ApplyFilterDialogEvent))
	case InputModeFindDialog:
		return found(a.findCtrl.DialogSpec(), &m.FindDialog.Focus, noQuit(a.findCtrl.ApplyDialogEvent))
	case InputModeMetaDialog:
		return found(dialog.MetaDialogView(m.MetaDialog), &m.MetaDialog.Focus, noQuit(a.metaCtrl.ApplyDialogEvent))
	case InputModeHelpView:
		if spec, _, ok := dialog.HelpDialogView(m.HelpView, layout, a.styles); ok {
			return found(spec, &m.HelpView.Focus, a.applyHelpEvent)
		}
	case InputModeThemeDialog:
		return found(a.themeDialogView(), &m.ThemeDialog.Focus, noQuit(a.applyThemeDialogEvent))
	case InputModeSortDialog:
		return found(dialog.SortDialogView(m.SortDialog), &m.SortDialog.Focus, noQuit(a.applySortDialogEvent))
	case InputModeListingFormatDialog:
		return found(dialog.ListingFormatView(m.ListingFormatDialog), &m.ListingFormatDialog.Focus, noQuit(a.applyListingFormatEvent))
	case InputModeConfigDialog:
		st := &m.ConfigDialog
		apply := noQuit(a.applyConfigDialogEvent) // routes the Yes/No sub-confirms itself
		switch {
		case st.EditStubConfirm:
			return found(dialog.ConfigEditStubView(), &st.EditStubConfirmFocus, apply)
		case st.ResetDefaultsConfirm:
			return found(dialog.ConfigResetDefaultsView(), &st.ResetDefaultsConfirmFocus, apply)
		}
		return found(dialog.ConfigDialogView(*st, w), &st.Focus, apply)
	case InputModePreviewSettingsDialog:
		return found(a.previewSettingsView(), &m.PreviewSettingsDialog.Focus, noQuit(a.applyPreviewSettingsEvent))
	case InputModeDebounceCalibrateDialog:
		return found(dialog.DebounceCalibrateView(m.DebounceCalibrateDialog), &m.DebounceCalibrateDialog.Focus, noQuit(a.applyDebounceCalibrateEvent))
	case InputModeHostKeyDialog:
		return found(dialog.HostKeyView(m.HostKeyDialog), &m.HostKeyDialog.Focus, noQuit(a.applyHostKeyEvent))
	case InputModeFileDialog:
		return found(a.dialogCtrl.FileDialogSpec(), m.FileDialog.SpecFocusRef(), noQuit(a.dialogCtrl.ApplyFileDialogEvent))
	case InputModeDialog:
		return a.modalDialogMouse()
	case InputModeDedupProgressDialog:
		return found(dialog.DedupProgressView(m.DedupSnapshot, m.UserHomeDir, time.Now()), &m.DedupProgressDialog.Focus, noQuit(a.dedupCtrl.ApplyProgressDialogEvent))
	case InputModeDedupReturnDialog:
		return found(ui.DedupReturnSpec(m), &m.DedupReturnDialog.Focus, noQuit(a.dedupCtrl.ApplyReturnDialogEvent))
	case InputModeCompareView:
		switch {
		case m.CompareFilterDialog.Open:
			return found(dialog.CompareFilterView(m.CompareFilterDialog), &m.CompareFilterDialog.Focus, noQuit(a.compareCtrl.ApplyFilterDialogEvent))
		case m.CompareMergeDialog.Open:
			return found(dialog.CompareMergeView(m.CompareMergeDialog, m.UserHomeDir), &m.CompareMergeDialog.Focus, noQuit(a.compareCtrl.ApplyMergeDialogEvent))
		}
	}
	return dialogMouse{}, false
}

// modalDialogMouse resolves the dialogs that share InputModeDialog, in handleDialogKey's order.
func (a *App) modalDialogMouse() (dialogMouse, bool) {
	m := &a.model
	found := func(spec dlg.Spec, focus *dlg.ID, apply func(dlg.Event) bool) (dialogMouse, bool) {
		return dialogMouse{spec, focus, apply}, true
	}
	switch {
	case m.ConflictDialog.Open:
		return found(dialog.ConflictDialogView(m.ConflictDialog, m.UserHomeDir), m.ConflictDialog.ConflictDialogFocus(), noQuit(a.jobsCtrl.ApplyBlockerDialogEvent))
	case m.TransferDialog.Open:
		return found(a.dialogCtrl.TransferDialogSpec(), &m.TransferDialog.Focus, noQuit(a.dialogCtrl.ApplyTransferEvent))
	case m.FlattenDialog.Open:
		return found(dialog.FlattenView(m.FlattenDialog), &m.FlattenDialog.Focus, noQuit(a.dialogCtrl.ApplyFlattenEvent))
	case m.QuitConfirm.Open:
		return found(dialog.QuitConfirmView(m.QuitConfirm), &m.QuitConfirm.Focus, a.applyQuitConfirmEvent)
	case m.DedupEmptyDirsConfirm.Open:
		return found(dialog.DedupEmptyDirsConfirmView(m.DedupEmptyDirsConfirm, ui.DialogListIconLeadingWidth(m.UseNerdfontIcons)), &m.DedupEmptyDirsConfirm.Focus, noQuit(a.applyDedupEmptyDirsConfirmEvent))
	case m.UndoConfirm.Open:
		return found(dialog.UndoConfirmView(m.UndoConfirm), &m.UndoConfirm.Focus, noQuit(a.applyUndoConfirmEvent))
	case m.StashRestoreDialog.Open:
		return found(dialog.StashRestoreView(m.StashRestoreDialog), &m.StashRestoreDialog.Focus, noQuit(a.applyStashRestoreEvent))
	}
	return dialogMouse{}, false
}

// handleDialogMouse applies a press or a wheel notch on the top-most dialog through the engine:
// dlg.Click / dlg.Wheel move the focus and return the event the matching key would produce, and
// the dialog's own event function applies it, so a click and its keyboard equivalent run the same
// code. OK / Cancel run activeDialogButtons when the dialog sets them. Presses that hit no widget
// (frame, labels, reserved regions) are swallowed.
func (a *App) handleDialogMouse(layout ui.Layout, x, y int, buttons, pressed tcell.ButtonMask) (quit, rendered bool) {
	wheel := wheelDelta(buttons, config.MouseWheelRows)
	if wheel == 0 && pressed&tcell.Button1 == 0 {
		return false, false
	}
	dm, ok := a.activeDialogMouse(layout)
	if !ok {
		return false, false
	}
	w, h := a.screen.Size()
	var ev dlg.Event
	if wheel != 0 {
		ev, ok = dlg.Wheel(dm.spec, w, h, dm.focus, x, y, wheel)
	} else {
		// Click reads double only on a list row, so the screen row identifies the target.
		ev, ok = dlg.Click(dm.spec, w, h, dm.focus, x, y, a.mouse.doubleClick("dialog", y))
	}
	if !ok {
		return false, false
	}
	a.deferDiskIdleSortOnUserActivity()
	a.deferMetaIdleSortOnUserActivity()
	if ev.ID == dlg.OK || ev.ID == dlg.Cancel {
		b := a.activeDialogButtons(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
		fn := b.OK
		if ev.ID == dlg.Cancel {
			fn = b.Cancel
		}
		if fn != nil {
			fn()
			a.render()
			return false, true
		}
	}
	if dm.apply(ev) {
		return true, false
	}
	a.render()
	return false, true
}
