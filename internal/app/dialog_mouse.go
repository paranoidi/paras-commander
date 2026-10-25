package app

import (
	"slices"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// dialogMouse is the top-most dlg dialog as the mouse sees it: its spec as drawn and the focus
// field its key handler reads.
type dialogMouse struct {
	spec  dlg.Spec
	focus *dlg.ID
}

// activeDialogMouse returns the top-most open dlg dialog. It switches on a.inputMode() in the same
// order as activeDialogButtons, and every dlg dialog adds one case. Each spec must be the one the
// renderer draws (same builder, same layout inputs), or clicks land on the wrong widget. Dialogs
// whose key handler keeps its own focus (message, pin) get a scratch focus.
func (a *App) activeDialogMouse() (dialogMouse, bool) {
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	found := func(spec dlg.Spec, focus *dlg.ID) (dialogMouse, bool) { return dialogMouse{spec, focus}, true }
	scratch := func(id dlg.ID) *dlg.ID { return &id }
	m := &a.model
	switch a.inputMode() {
	case InputModeElevateDialog:
		return found(dialog.ElevateView(m.ElevateDialog), &m.ElevateDialog.Focus)
	case InputModeGroupSelect:
		return found(a.groupSelectView(), &m.GroupSelect.Focus)
	case InputModeCommandOutputDialog:
		if spec, _, ok := dialog.CommandOutputDialogView(layout, m.CommandOutputDialog, a.styles); ok {
			return found(spec, &m.CommandOutputDialog.Focus)
		}
	case InputModeMessageDialog:
		return found(dialog.MessageDialogView(m.MessageDialog, layout), scratch(dlg.OK))
	case InputModePathPicker:
		return found(a.dialogCtrl.PathPickerView(), &m.PathPicker.Focus)
	case InputModeHistoryDialog:
		return found(a.historyView(), &m.HistoryDialog.Focus)
	case InputModePinDialog:
		return found(a.pinCtrl.DialogView(), scratch(dialog.PinFilter))
	case InputModeSFTPConnectDialog:
		return found(a.sftpConnectView(), &m.SFTPConnectDialog.Focus)
	case InputModeFilterDialog:
		return found(a.dialogCtrl.FilterDialogView(), &m.FilterDialog.Focus)
	case InputModeFindDialog:
		return found(a.findCtrl.DialogSpec(), &m.FindDialog.Focus)
	case InputModeMetaDialog:
		return found(dialog.MetaDialogView(m.MetaDialog), &m.MetaDialog.Focus)
	case InputModeHelpView:
		if spec, _, ok := dialog.HelpDialogView(m.HelpView, layout, a.styles); ok {
			return found(spec, &m.HelpView.Focus)
		}
	case InputModeThemeDialog:
		return found(a.themeDialogView(), &m.ThemeDialog.Focus)
	case InputModeSortDialog:
		return found(dialog.SortDialogView(m.SortDialog), &m.SortDialog.Focus)
	case InputModeListingFormatDialog:
		return found(dialog.ListingFormatView(m.ListingFormatDialog), &m.ListingFormatDialog.Focus)
	case InputModeConfigDialog:
		st := &m.ConfigDialog
		switch {
		case st.EditStubConfirm:
			return found(dialog.ConfigEditStubView(), &st.EditStubConfirmFocus)
		case st.ResetDefaultsConfirm:
			return found(dialog.ConfigResetDefaultsView(), &st.ResetDefaultsConfirmFocus)
		}
		return found(dialog.ConfigDialogView(*st, w), &st.Focus)
	case InputModePreviewSettingsDialog:
		return found(a.previewSettingsView(), &m.PreviewSettingsDialog.Focus)
	case InputModeDebounceCalibrateDialog:
		return found(dialog.DebounceCalibrateView(m.DebounceCalibrateDialog), &m.DebounceCalibrateDialog.Focus)
	case InputModeHostKeyDialog:
		return found(dialog.HostKeyView(m.HostKeyDialog), &m.HostKeyDialog.Focus)
	case InputModeFileDialog:
		return found(a.dialogCtrl.FileDialogSpec(), m.FileDialog.SpecFocusRef())
	case InputModeDialog:
		return a.modalDialogMouse()
	case InputModeDedupProgressDialog:
		return found(dialog.DedupProgressView(m.DedupSnapshot, m.UserHomeDir, time.Now()), &m.DedupProgressDialog.Focus)
	case InputModeDedupReturnDialog:
		return found(ui.DedupReturnSpec(m), &m.DedupReturnDialog.Focus)
	case InputModeCompareView:
		switch {
		case m.CompareFilterDialog.Open:
			return found(dialog.CompareFilterView(m.CompareFilterDialog), &m.CompareFilterDialog.Focus)
		case m.CompareMergeDialog.Open:
			return found(dialog.CompareMergeView(m.CompareMergeDialog, m.UserHomeDir), &m.CompareMergeDialog.Focus)
		}
	}
	return dialogMouse{}, false
}

// modalDialogMouse resolves the dialogs that share InputModeDialog, in handleDialogKey's order.
func (a *App) modalDialogMouse() (dialogMouse, bool) {
	m := &a.model
	found := func(spec dlg.Spec, focus *dlg.ID) (dialogMouse, bool) { return dialogMouse{spec, focus}, true }
	switch {
	case m.ConflictDialog.Open:
		return found(dialog.ConflictDialogView(m.ConflictDialog, m.UserHomeDir), m.ConflictDialog.ConflictDialogFocus())
	case m.TransferDialog.Open:
		return found(a.dialogCtrl.TransferDialogSpec(), &m.TransferDialog.Focus)
	case m.FlattenDialog.Open:
		return found(dialog.FlattenView(m.FlattenDialog), &m.FlattenDialog.Focus)
	case m.QuitConfirm.Open:
		return found(dialog.QuitConfirmView(m.QuitConfirm), &m.QuitConfirm.Focus)
	case m.DedupEmptyDirsConfirm.Open:
		return found(dialog.DedupEmptyDirsConfirmView(m.DedupEmptyDirsConfirm, ui.DialogListIconLeadingWidth(m.UseNerdfontIcons)), &m.DedupEmptyDirsConfirm.Focus)
	case m.UndoConfirm.Open:
		return found(dialog.UndoConfirmView(m.UndoConfirm), &m.UndoConfirm.Focus)
	case m.StashRestoreDialog.Open:
		return found(dialog.StashRestoreView(m.StashRestoreDialog), &m.StashRestoreDialog.Focus)
	}
	return dialogMouse{}, false
}

// handleDialogMouse turns a press on the top-most dialog into the keys its handler already takes,
// so the mouse adds no second code path for any dialog: OK / Cancel run activeDialogButtons, any
// other button is focused and pressed with Enter, a radio or checkbox is focused and toggled with
// Space, a list row is reached with Up/Down from the cursor (Enter on a double click), an input
// is focused and its cursor moved with Home and Right, and the wheel over a list sends Up/Down.
// Presses that hit no widget (frame, labels, reserved regions) are swallowed.
func (a *App) handleDialogMouse(x, y int, buttons, pressed tcell.ButtonMask) (quit, rendered bool) {
	wheel := buttons & (tcell.WheelUp | tcell.WheelDown)
	if wheel == 0 && pressed&tcell.Button1 == 0 {
		return false, false
	}
	dm, ok := a.activeDialogMouse()
	if !ok {
		return false, false
	}
	w, h := a.screen.Size()
	t, hit := dlg.TargetAt(dm.spec, w, h, x, y)
	if !hit {
		return false, false
	}
	key := func(k tcell.Key) *tcell.EventKey { return tcell.NewEventKey(k, 0, tcell.ModNone) }
	step := func(delta int) []*tcell.EventKey {
		k := tcell.KeyDown
		if delta < 0 {
			k, delta = tcell.KeyUp, -delta
		}
		return slices.Repeat([]*tcell.EventKey{key(k)}, delta)
	}
	if wheel != 0 {
		if t.Kind != dlg.TargetList {
			return false, false
		}
		*dm.focus = t.Focus
		delta := config.MouseWheelRows
		if wheel&tcell.WheelUp != 0 {
			delta = -delta
		}
		return a.replayDialogKeys(step(delta))
	}
	switch t.Kind {
	case dlg.TargetButton:
		if t.ID == dlg.OK || t.ID == dlg.Cancel {
			b := a.activeDialogButtons(key(tcell.KeyEnter))
			fn := b.OK
			if t.ID == dlg.Cancel {
				fn = b.Cancel
			}
			if fn != nil {
				fn()
				a.render()
				return false, true
			}
		}
		*dm.focus = t.ID
		return a.replayDialogKeys([]*tcell.EventKey{key(tcell.KeyEnter)})
	case dlg.TargetOption:
		*dm.focus = t.ID
		return a.replayDialogKeys([]*tcell.EventKey{tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone)})
	case dlg.TargetInput:
		*dm.focus = t.ID
		right := slices.Repeat([]*tcell.EventKey{key(tcell.KeyRight)}, t.TextCol)
		return a.replayDialogKeys(append([]*tcell.EventKey{key(tcell.KeyHome)}, right...))
	default: // dlg.TargetList
		*dm.focus = t.Focus
		keys := step(t.Row - t.Cursor)
		if a.mouse.doubleClick("dialog-list:"+string(t.ID), t.Row) {
			keys = append(keys, key(tcell.KeyEnter))
		}
		return a.replayDialogKeys(keys)
	}
}

// replayDialogKeys feeds keys through handleKey while the dialog that took the press stays on top.
func (a *App) replayDialogKeys(keys []*tcell.EventKey) (quit, rendered bool) {
	mode := a.inputMode()
	for _, ev := range keys {
		if a.inputMode() != mode {
			break
		}
		if quit, _ := a.handleKey(ev); quit {
			return true, false
		}
	}
	a.render()
	return false, true
}
