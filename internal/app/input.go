package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// InputMode describes the active input context for key routing.
type InputMode int

const (
	InputModeNormal InputMode = iota
	InputModeMessageDialog
	InputModeThemeDialog
	InputModeSortDialog
	InputModeListingFormatDialog
	InputModeConfigDialog
	InputModeDebounceCalibrateDialog
	InputModeGroupSelect
	InputModeMenu
	InputModeFilter
	InputModeFileDialog
	InputModeDialog
	InputModeJobsView
	InputModeCommandsView
	InputModeMessagesView
	InputModeCompareView
	InputModeDedupProgressDialog
	InputModeDedupView
	InputModeFilePreviewView
	InputModePathPicker
	InputModeHistoryDialog
	InputModeFindDialog
	InputModeMetaDialog
	InputModeLeaderMenu
	InputModeHelpView
	InputModeHostKeyDialog
	InputModeSFTPConnectDialog
	InputModeCommandOutputDialog
)

// viewActiveForInput reports whether vm is the active view and eligible to receive
// view-specific key handling (no blocking dialog keys, no quick-filter UI in front).
func (a *App) viewActiveForInput(vm ui.ViewMode) bool {
	return a.model.ViewMode == vm && !a.model.AuxiliaryViewDialogKeysBlocked() && !a.inQuickFilterUI()
}

func (a *App) inputMode() InputMode {
	switch {
	case a.model.CommandOutputDialog.Open:
		return InputModeCommandOutputDialog
	case a.model.MessageDialog.Open:
		return InputModeMessageDialog
	case a.model.PathPicker.Open:
		return InputModePathPicker
	case a.model.HistoryDialog.Open:
		return InputModeHistoryDialog
	case a.model.SFTPConnectDialog.Open:
		return InputModeSFTPConnectDialog
	case a.model.GroupSelect.Open:
		return InputModeGroupSelect
	case a.model.FindDialog.Open:
		return InputModeFindDialog
	case a.model.MetaDialog.Open:
		return InputModeMetaDialog
	case a.model.LeaderMenu.Open:
		return InputModeLeaderMenu
	case a.model.HelpView.Open:
		return InputModeHelpView
	case a.model.ThemeDialog.Open:
		return InputModeThemeDialog
	case a.model.SortDialog.Open:
		return InputModeSortDialog
	case a.model.ListingFormatDialog.Open:
		return InputModeListingFormatDialog
	case a.model.ConfigDialog.Open:
		return InputModeConfigDialog
	case a.model.DebounceCalibrateDialog.Open:
		return InputModeDebounceCalibrateDialog
	case a.model.HostKeyDialog.Open:
		return InputModeHostKeyDialog
	case a.model.FileDialog.Open:
		return InputModeFileDialog
	case a.model.DedupProgressDialog.Open:
		return InputModeDedupProgressDialog
	case a.viewActiveForInput(ui.ViewCompare):
		return InputModeCompareView
	case a.viewActiveForInput(ui.ViewDedup):
		return InputModeDedupView
	case a.viewActiveForInput(ui.ViewFilePreview):
		return InputModeFilePreviewView
	case a.viewActiveForInput(ui.ViewCommands):
		return InputModeCommandsView
	case a.viewActiveForInput(ui.ViewMessages):
		return InputModeMessagesView
	case a.viewActiveForInput(ui.ViewJobs):
		return InputModeJobsView
	case a.model.TransferDialog.Open, a.model.FlattenDialog.Open, a.model.ConflictDialog.Open, a.model.QuitConfirm.Open, a.model.DedupEmptyDirsConfirm.Open, a.model.StashRestoreDialog.Open:
		return InputModeDialog
	case a.model.Menu.Open:
		return InputModeMenu
	case a.inQuickFilterUI():
		return InputModeFilter
	default:
		return InputModeNormal
	}
}

// activeFooterKeys returns F-key hints filtered by current app state.
// Dialogs show Esc (close) first, then other keys (e.g. F10 quit). Normal mode shows all.
func (a *App) activeFooterKeys() []menu.FunctionKey {
	// Focused terminal panel receives every key (F5 is the shell's, not Copy) — a
	// populated footer would lie about who handles the F-keys.
	if a.terminalPanelHasKeyFocus() {
		return nil
	}
	if a.model.MessageDialog.Open {
		return footerWithEscClose([]menu.FunctionKey{
			{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
		})
	}
	if a.model.DedupProgressDialog.Open {
		return footerWithEscClose([]menu.FunctionKey{
			{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
		})
	}
	if a.model.ThemeDialog.Open {
		return footerWithEscClose([]menu.FunctionKey{
			{Key: tcell.KeyF5, KeyLabel: "F5", Hint: "Reload"},
			{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
		})
	}
	if a.model.LeaderMenu.Open {
		if a.model.LeaderMenu.UserMenu {
			keys := footerWithEscClose([]menu.FunctionKey{
				menu.FunctionKeyEditConfig,
				{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
			})
			return keys
		}
		if a.model.LeaderMenu.CopyMenu {
			return footerWithEscClose([]menu.FunctionKey{
				{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
			})
		}
		if a.model.LeaderMenu.PreviewMenu {
			return footerWithEscClose([]menu.FunctionKey{
				{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
			})
		}
		return footerWithEscClose([]menu.FunctionKey{
			menu.FunctionKeyLeaderMenuToggleChords,
			{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
		})
	}
	if a.model.FindDialog.Open {
		rest := []menu.FunctionKey{{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"}}
		rest = append(findDialogOverlayFooterKeys(a.keys.FindDialog), rest...)
		return footerWithEscClose(rest)
	}
	if a.model.HistoryDialog.Open {
		rest := []menu.FunctionKey{{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"}}
		rest = append(historyDialogOverlayFooterKeys(a.keys.HistoryDialog, a.model.HistoryDialog.BothPanels), rest...)
		return footerWithEscClose(rest)
	}
	if a.model.PathPicker.Open || a.model.MetaDialog.Open {
		return a.pathPickerMetaFooterKeys()
	}
	if a.model.CommandOutputDialog.Open {
		return footerWithEscClose([]menu.FunctionKey{
			{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
		})
	}
	if a.model.AnyModalOpen() {
		return a.primaryModalFooterKeys()
	}
	if a.model.Menu.Open {
		// Menu open: Esc closes menu / pulldown; F9 and F10 as before.
		return footerWithEscClose([]menu.FunctionKey{
			{Key: tcell.KeyF9, KeyLabel: "F9", Hint: "Menu"},
			{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
		})
	}
	if keys, ok := a.auxiliaryViewFooterKeys(); ok {
		return keys
	}
	if a.model.HelpView.Open {
		return footerWithEscClose([]menu.FunctionKey{
			{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
		})
	}
	if a.model.ViewMode == ui.ViewBrowser &&
		a.model.ActiveSubFocus == ui.SubFocusSelectionsStrip &&
		!a.inQuickFilterUI() {
		return menu.FunctionKeysSelectionsStripView(a.keys.Global.MenuBindingLabel(keymap.ActionPanelClearSelection))
	}
	return menu.FunctionKeys
}

// pathPickerMetaFooterKeys builds footer hints for the PathPicker / MetaDialog overlay branch.
func (a *App) pathPickerMetaFooterKeys() []menu.FunctionKey {
	rest := []menu.FunctionKey{{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"}}
	if a.model.MetaDialog.Open {
		rest = append([]menu.FunctionKey{menu.FunctionKeyEditConfig}, rest...)
	}
	if a.dialogCtrl.BookmarkDialogDeleteFooterEligible() {
		if lbl := a.keys.BookmarkDialog.MenuBindingLabel(keymap.ActionBookmarkDelete); lbl != "" {
			rest = append([]menu.FunctionKey{{Key: tcell.KeyF8, KeyLabel: lbl, Hint: "Delete bookmark"}}, rest...)
		}
	}
	if a.dialogCtrl.BookmarkDialogOpenOtherFooterEligible() {
		if lbl := a.keys.BookmarkDialog.MenuBindingLabel(keymap.ActionBookmarkOpenOther); lbl != "" {
			rest = append([]menu.FunctionKey{{KeyLabel: lbl, Hint: "Open other"}}, rest...)
		}
	}
	return footerWithEscClose(rest)
}

// primaryModalFooterKeys builds footer hints for the primary-modal branch (sort/listing-format/config/
// group-select/file-dialog/SFTP-connect/path-picker/history/find/meta/user-menu/compare-merge dialogs).
func (a *App) primaryModalFooterKeys() []menu.FunctionKey {
	rest := []menu.FunctionKey{{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"}}
	if hints := a.dialogCtrl.FlattenDialogOverlayFooterKeys(a.keys.FlattenDialog); len(hints) > 0 {
		rest = append(hints, rest...)
	}
	if hints := a.dialogCtrl.TransferDialogOverlayFooterKeys(a.keys.TransferDialog); len(hints) > 0 {
		rest = append(hints, rest...)
	}
	if a.dialogCtrl.PathPickerHostFooterEligible() {
		if lbl := a.keys.Global.MenuBindingLabel(keymap.ActionBookmarkOpen); lbl != "" {
			rest = append([]menu.FunctionKey{{KeyLabel: lbl, Hint: "Bookmarks"}}, rest...)
		}
	}
	if a.dialogCtrl.MkdirDialogExtractFooterEligible() {
		if lbl := a.keys.MkdirDialog.MenuBindingLabel(keymap.ActionFileMkdirExtractCommonName); lbl != "" {
			rest = append([]menu.FunctionKey{{Key: tcell.KeyF7, KeyLabel: lbl, Hint: "Extract common name"}}, rest...)
		}
	}
	if a.dialogCtrl.DialogInputRestoreFooterEligible() {
		if lbl := a.keys.DialogInput.MenuBindingLabel(keymap.ActionDialogInputRestoreDefault); lbl != "" {
			rest = append([]menu.FunctionKey{{KeyLabel: lbl, Hint: "Default"}}, rest...)
		}
	}
	if a.dialogCtrl.MassRenameEditorFooterEligible() {
		rest = append([]menu.FunctionKey{{Key: tcell.KeyF4, KeyLabel: "F4", Hint: "Editor"}}, rest...)
	}
	if a.dialogCtrl.MassRenameDeletePatternFooterEligible() {
		if lbl := a.keys.MassRenameDialog.MenuBindingLabel(keymap.ActionFileMassRenameDeletePattern); lbl != "" {
			rest = append([]menu.FunctionKey{{Key: tcell.KeyF8, KeyLabel: lbl, Hint: "Delete pattern"}}, rest...)
		}
	}
	if a.dialogCtrl.MassRenameSavePatternFooterEligible() {
		if lbl := a.keys.MassRenameDialog.MenuBindingLabel(keymap.ActionFileMassRenameSavePattern); lbl != "" {
			rest = append([]menu.FunctionKey{{Key: tcell.KeyF5, KeyLabel: lbl, Hint: "Save pattern"}}, rest...)
		}
	}
	if a.dialogCtrl.MassRenameHistoryFooterEligible() {
		if lbl := a.keys.MassRenameDialog.MenuBindingLabel(keymap.ActionFileMassRenameHistory); lbl != "" {
			rest = append([]menu.FunctionKey{{Key: tcell.KeyF3, KeyLabel: lbl, Hint: "History"}}, rest...)
		}
	}
	if a.dialogCtrl.MassRenameLoadPatternFooterEligible() {
		if lbl := a.keys.MassRenameDialog.MenuBindingLabel(keymap.ActionFileMassRenameLoadPattern); lbl != "" {
			rest = append([]menu.FunctionKey{{Key: tcell.KeyF2, KeyLabel: lbl, Hint: "Load pattern"}}, rest...)
		}
	}
	if a.dialogCtrl.RenameDialogFooterEligible() {
		if a.dialogCtrl.RenameEncodingFooterEligible() {
			if lbl := a.keys.RenameDialog.MenuBindingLabel(keymap.ActionFileRenameOpenEncoding); lbl != "" {
				rest = append([]menu.FunctionKey{{Key: tcell.KeyF4, KeyLabel: lbl, Hint: "Encoding"}}, rest...)
			}
		}
		if lbl := a.keys.RenameDialog.MenuBindingLabel(keymap.ActionFileRenameOpenSlugify); lbl != "" {
			rest = append([]menu.FunctionKey{{Key: tcell.KeyF3, KeyLabel: lbl, Hint: "Slugify"}}, rest...)
		}
		if lbl := a.keys.RenameDialog.MenuBindingLabel(keymap.ActionFileRenameOpenSanitize); lbl != "" {
			rest = append([]menu.FunctionKey{{Key: tcell.KeyF2, KeyLabel: lbl, Hint: "Sanitize"}}, rest...)
		}
	}
	return footerWithEscClose(rest)
}

// auxiliaryViewFooterKeys builds footer hints for FilePreview/Compare/Dedup/Commands/Messages/Jobs
// views. Returns ok=false when none of those views is active so the caller falls through.
func (a *App) auxiliaryViewFooterKeys() ([]menu.FunctionKey, bool) {
	if a.model.ViewMode == ui.ViewFilePreview && !a.inQuickFilterUI() {
		if a.model.FilePreviewThemePicker.Open {
			return menu.FunctionKeysFilePreviewStylePicker(), true
		}
		return menu.FunctionKeysFilePreviewView(a.model.FullscreenFilePreviewRawMarkdown, a.launchedFileViewer), true
	}
	if a.model.ViewMode == ui.ViewCompare && !a.inQuickFilterUI() {
		rest := compareViewFooterKeys(a.keys.Compare, a.model.CompareView.Filter)
		rest = append(rest, menu.FunctionKey{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"})
		out := footerWithEscClose(rest)
		f1 := menu.FunctionKey{Key: tcell.KeyF1, KeyLabel: "F1", Hint: "Help"}
		out = append(out[:1], append([]menu.FunctionKey{f1}, out[1:]...)...)
		return out, true
	}
	if a.model.ViewMode == ui.ViewDedup && !a.inQuickFilterUI() {
		rest := dedupViewFooterKeys(a.keys.Global, a.keys.Dedup, a.model.DedupView.TreeDirs)
		rest = append(rest,
			menu.FunctionKey{Key: tcell.KeyF9, KeyLabel: "F9", Hint: "Menu"},
			menu.FunctionKey{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
		)
		out := footerWithEscClose(rest)
		f1 := menu.FunctionKey{Key: tcell.KeyF1, KeyLabel: "F1", Hint: "Help"}
		out = append(out[:1], append([]menu.FunctionKey{f1}, out[1:]...)...)
		return out, true
	}
	if a.model.ViewMode == ui.ViewCommands && !a.inQuickFilterUI() {
		return menu.FunctionKeysCommandsView(), true
	}
	if a.model.ViewMode == ui.ViewMessages && !a.inQuickFilterUI() {
		return menu.FunctionKeysMessagesView(), true
	}
	if a.model.ViewMode == ui.ViewJobs && !a.inQuickFilterUI() {
		return menu.FunctionKeysJobsView(), true
	}
	return nil, false
}

func footerWithEscClose(rest []menu.FunctionKey) []menu.FunctionKey {
	out := make([]menu.FunctionKey, 0, len(rest)+1)
	out = append(out, menu.FooterEscClose)
	out = append(out, rest...)
	return out
}

func findDialogOverlayFooterKeys(keys *keymap.Map) []menu.FunctionKey {
	if keys == nil {
		return nil
	}
	var out []menu.FunctionKey
	if lbl := keys.MenuBindingLabel(keymap.ActionFindView); lbl != "" {
		out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "View"})
	}
	if lbl := keys.MenuBindingLabel(keymap.ActionFindUnselectAll); lbl != "" {
		out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Unselect all"})
	}
	if lbl := keys.MenuBindingLabel(keymap.ActionFindSelectAll); lbl != "" {
		out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Select all"})
	}
	if lbl := keys.MenuBindingLabel(keymap.ActionFindSelectGroup); lbl != "" {
		out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Select group"})
	}
	if lbl := keys.MenuBindingLabel(keymap.ActionFindUnselectGroup); lbl != "" {
		out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Unselect group"})
	}
	if lbl := keys.MenuBindingLabel(keymap.ActionFindOpenInPrimary); lbl != "" {
		out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Open ◄"})
	}
	if lbl := keys.MenuBindingLabel(keymap.ActionFindOpenInSecondary); lbl != "" {
		out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Open ►"})
	}
	return out
}

func (a *App) prepareGlobalQuitShortcutCleanup() {
	a.clearPanelSyncFollowNavCoalesce()
	a.previewCtrl.ClearNavCoalesces()
	a.clearCursorNameHintNavCoalesce()
	if a.inQuickFilterUI() {
		a.cancelActiveQuickFilter()
	}
}

// handleGlobalKeyIntercepts handles the pre-dispatch global key intercepts that apply
// regardless of input mode: terminal-panel focus, F10/Shift-F10 quit, quit-immediate,
// nav-coalesce clearing, disk-usage clear, jobs-answer-blocker, and global show-help.
// handled reports whether handleKey should return (quit, rendered) immediately.
func (a *App) handleGlobalKeyIntercepts(event *tcell.EventKey, resolvedAction string) (handled, quit, rendered bool) {
	// Focused terminal panel owns every key before the global intercepts below —
	// F10 must reach htop, F1 must reach the shell, only [terminal] chords are ours.
	if a.terminalPanelHasKeyFocus() {
		return true, false, a.handleTerminalPanelKey(event)
	}
	// Global F10 quit - works from any mode, any dialog, any menu.
	// Shift+F10 defaults to app.quit-immediate and must not fall through to plain F10.
	if event.Key() == tcell.KeyF10 {
		if event.Modifiers()&tcell.ModShift != 0 {
			if id, ok := a.keys.Global.Lookup(event); ok && id == keymap.ActionAppQuitImmediate {
				a.prepareGlobalQuitShortcutCleanup()
				return true, a.handleQuitImmediate(), false
			}
		} else {
			a.prepareGlobalQuitShortcutCleanup()
			if a.model.QuitConfirm.Open {
				a.model.QuitConfirm = dialog.QuitConfirmState{}
				a.stopWorker()
				return true, true, false
			}
			return true, a.handleQuit(), false
		}
	}

	if id, ok := a.keys.Global.Lookup(event); ok && id == keymap.ActionAppQuitImmediate {
		a.prepareGlobalQuitShortcutCleanup()
		return true, a.handleQuitImmediate(), false
	}

	if !a.panelSyncFollowHeldListNav(resolvedAction, event) {
		a.clearPanelSyncFollowNavCoalesce()
		a.previewCtrl.ClearQuickViewNavCoalesce()
		a.clearCursorNameHintNavCoalesce()
	}
	if !a.previewCtrl.CarouselPreviewHeldListNav(resolvedAction, event) {
		a.previewCtrl.ClearCarouselPreviewNavCoalesce()
	}
	if !a.model.ModalDialogOpen() {
		if resolvedAction == keymap.ActionPanelDiskUsageClear {
			a.clearAllDiskUsageData()
			a.render()
			return true, false, true
		}
	}

	if resolvedAction == keymap.ActionJobsAnswerBlocker {
		if rendered := a.jobsCtrl.HandleAnswerBlockerKey(); rendered {
			a.render()
			return true, false, true
		}
		return true, false, false
	}

	// Global show-help (F1 by default). Toggles: closes if already open,
	// otherwise closes menu or quick filter first and opens.
	if resolvedAction == keymap.ActionAppShowHelp {
		if a.model.HelpView.Open {
			a.closeHelpDialog()
			a.render()
			return true, false, true
		}
		// Do not open help from other modal dialogs.
		if a.model.ModalDialogOpen() {
			return true, false, false
		}
		if a.model.Menu.Open {
			a.closeMenu()
		}
		if a.inQuickFilterUI() {
			a.cancelActiveQuickFilter()
		}
		a.openHelpDialog()
		a.render()
		return true, false, true
	}

	return false, false, false
}

func (a *App) handleKey(event *tcell.EventKey) (quit bool, rendered bool) {
	a.deferDiskIdleSortOnUserActivity()
	resolvedAction := a.actionFromKeyEvent(event)
	if handled, iquit, irendered := a.handleGlobalKeyIntercepts(event, resolvedAction); handled {
		return iquit, irendered
	}

	switch a.inputMode() {
	case InputModeCommandOutputDialog:
		a.commandsCtrl.HandleOutputDialogKey(event)
		a.render()
		return false, true
	case InputModeMessageDialog:
		a.handleMessageDialogKey(event)
		a.render()
		return false, true
	case InputModePathPicker:
		a.dialogCtrl.HandlePathPickerKey(event)
		a.render()
		return false, true
	case InputModeHistoryDialog:
		a.handleHistoryDialogKey(event)
		a.render()
		return false, true
	case InputModeSFTPConnectDialog:
		a.handleSFTPConnectDialogKey(event)
		a.render()
		return false, true
	case InputModeFindDialog:
		wasOpen := a.model.FindDialog.Open
		gsWasOpen := a.model.GroupSelect.Open
		primaryPath := a.model.Primary.PathString()
		secondaryPath := a.model.Secondary.PathString()
		a.findCtrl.HandleDialogKey(event)
		panelsChanged := a.model.Primary.PathString() != primaryPath || a.model.Secondary.PathString() != secondaryPath
		if !wasOpen || !a.model.FindDialog.Open || (!gsWasOpen && a.model.GroupSelect.Open) || panelsChanged {
			a.render()
		} else if !a.paintFindDialogOverlay() {
			a.render()
		}
		return false, true
	case InputModeMetaDialog:
		a.metaCtrl.HandleDialogKey(event)
		a.render()
		return false, true
	case InputModeLeaderMenu:
		if resolvedAction == keymap.ActionAppLeaderMenu && a.builtinLeaderMenuOpen() {
			a.closeLeaderMenu()
			a.render()
			return false, true
		}
		if resolvedAction == keymap.ActionFileViewMenu && a.previewLeaderMenuOpen() {
			a.closeLeaderMenu()
			a.render()
			return false, true
		}
		quit := a.handleLeaderMenuKey(event)
		if !quit {
			a.render()
		}
		return quit, true
	case InputModeHelpView:
		quit := a.handleHelpDialogKey(event)
		a.render()
		return quit, true
	case InputModeThemeDialog:
		a.handleThemeDialogKey(event)
		a.render()
		return false, true
	case InputModeSortDialog:
		a.handleSortDialogKey(event)
		a.render()
		return false, true
	case InputModeListingFormatDialog:
		a.handleListingFormatDialogKey(event)
		a.render()
		return false, true
	case InputModeConfigDialog:
		a.handleConfigDialogKey(event)
		a.render()
		return false, true
	case InputModeDebounceCalibrateDialog:
		a.handleDebounceCalibrateDialogKey(event)
		a.render()
		return false, true
	case InputModeGroupSelect:
		a.handleGroupSelectKey(event)
		a.render()
		return false, true
	case InputModeHostKeyDialog:
		_ = a.handleHostKeyDialogKey(event)
		a.render()
		return false, true
	case InputModeFileDialog:
		before := a.dialogCtrl.FileDialogRect()
		quit := a.dialogCtrl.HandleFileDialogKey(event)
		// Overlay-repaint only the dialog rect when the dialog stayed open, no sub-modal
		// opened (a path picker flips inputMode away), and the geometry is unchanged.
		// Anything else (open/close, phase/mode switch, geometry change) needs a full render.
		if a.model.FileDialog.Open && a.inputMode() == InputModeFileDialog && a.dialogCtrl.FileDialogRect() == before {
			if !a.paintFileDialogOverlay() {
				a.render()
			}
		} else {
			a.render()
		}
		return quit, true
	case InputModeJobsView:
		quit := a.jobsCtrl.HandleJobsViewKey(event)
		a.render()
		return quit, true
	case InputModeCommandsView:
		quit := a.commandsCtrl.HandleViewKey(event)
		a.render()
		return quit, true
	case InputModeCompareView:
		quit := a.handleCompareViewKey(event)
		a.render()
		return quit, true
	case InputModeDedupProgressDialog:
		a.dedupCtrl.HandleProgressDialogKey(event)
		a.render()
		return false, true
	case InputModeDedupView:
		quit := a.handleDedupViewKey(event)
		a.render()
		return quit, true
	case InputModeMessagesView:
		quit := a.handleMessagesViewKey(event)
		a.render()
		return quit, true
	case InputModeFilePreviewView:
		quit := a.previewCtrl.HandleFilePreviewViewKey(event)
		a.render()
		return quit, true
	case InputModeDialog:
		quit := a.handleDialogKey(event)
		a.render()
		return quit, true
	case InputModeMenu:
		quit := a.handleMenuKey(event)
		a.render()
		return quit, true
	case InputModeFilter:
		if fQuit, fRendered, handled := a.handleFilterLeaderKey(event, resolvedAction); handled {
			return fQuit, fRendered
		}
		// If neither function key nor filter key, fall through to action dispatch.
	default: // InputModeNormal
		if a.model.ViewMode == ui.ViewBrowser && a.model.ActiveSubFocus == ui.SubFocusSelectionsStrip {
			if a.handleSelectionsStripKey(event) {
				a.render()
				return false, true
			}
		}
		// Plain printable keys start the quick filter.
		if a.shouldStartFilter(event) {
			a.handleFilterKey(event)
			a.render()
			return false, true
		}
	}
	// Action dispatch path for normal mode and unhandled filter keys.
	nextAction := resolvedAction
	if nextAction == "" {
		// Alt+letter opens the corresponding pulldown menu directly.
		if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
			if a.model.ViewMode == ui.ViewBrowser && a.model.ActiveSubFocus == ui.SubFocusSelectionsStrip {
				return false, false
			}
			if a.openMenuByShortcut(event.Rune()) {
				a.render()
				return false, true
			}
		}
		return false, false
	}
	if isNavParentBackspaceEvent(event, nextAction) && a.navParentBackspaceGuarded.Load() {
		a.armNavParentBackspaceGuard() // keep guard alive while key is still held
		return false, false
	}
	quit, rendered = a.finishResolvedKeyboardAction(nextAction)
	if rendered {
		if panelSyncFollowListNavAction(nextAction) && a.browserListNavPartialRenderEligible() {
			a.renderBrowserListNavUpdate()
		} else {
			a.render()
		}
	}
	return quit, rendered
}

// handleFilterLeaderKey handles the InputModeFilter case in handleKey. handled=false means
// neither a function key nor a filter key matched, so the caller falls through to normal
// action dispatch.
func (a *App) handleFilterLeaderKey(event *tcell.EventKey, resolvedAction string) (quit, rendered, handled bool) {
	// Function keys in filter mode dismiss the filter and run the menu action.
	if _, ok := menu.FunctionKeyLabelByKey(event.Key()); ok {
		quit := a.handleQuickFilterFunctionKey(event)
		a.render()
		return quit, true, true
	}
	// Bound actions (same keymap as normal browser mode) dismiss the filter unless
	// the key is filter-local (typing, match cycling, Insert, etc.).
	if resolvedAction != "" && !a.quickFilterRetainsKey(event, resolvedAction) {
		a.cancelActiveQuickFilter()
		fqQuit, fqRendered := a.finishResolvedKeyboardAction(resolvedAction)
		if fqRendered {
			a.render()
		}
		return fqQuit, fqRendered, true
	}
	// Filter keys (printable, navigation, etc.) update the filter.
	if a.shouldHandleFilterKey(event) {
		a.handleFilterKey(event)
		a.render()
		return false, true, true
	}
	return false, false, false
}

// quickFilterRetainsKey reports keys that stay on the quick-filter input path even when
// the same chord is bound to a panel action (match cycling, Insert, query editing).
func (a *App) quickFilterRetainsKey(event *tcell.EventKey, resolvedAction string) bool {
	switch event.Key() {
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyInsert, tcell.KeyHome, tcell.KeyEnd:
		return true
	}
	if keymap.IsPlainPrintableRune(event) {
		// Space is always filter text while typing, even though it's bound to
		// panel.toggle-tree — otherwise a space in the query would expand/collapse
		// the selected row and drop out of the filter.
		if event.Rune() == ' ' {
			return true
		}
		// Leader menu key (:) is filter text while typing, not a menu dismiss/open chord.
		if resolvedAction == keymap.ActionAppLeaderMenu {
			return true
		}
		return resolvedAction == ""
	}
	f := a.activeQuickFilter()
	switch event.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if event.Modifiers()&tcell.ModCtrl != 0 {
			return f.Editing || f.Active
		}
		return f.Editing || f.Query != ""
	case tcell.KeyEnter:
		return f.Editing && strings.TrimSpace(f.Query) == ""
	default:
		return false
	}
}

// finishResolvedKeyboardAction runs copy/move/quit or dispatch for a resolved keybinding.
// The second return is whether the caller should render; quit does not render (matches prior handleKey behavior).
func (a *App) finishResolvedKeyboardAction(nextAction string) (quit bool, rendered bool) {
	switch nextAction {
	case keymap.ActionCopy:
		a.dialogCtrl.ActivateCopyAction()
		return false, true
	case keymap.ActionMove:
		a.dialogCtrl.ActivateMoveAction()
		return false, true
	case keymap.ActionAppQuit:
		return a.handleQuit(), false
	case keymap.ActionAppQuitImmediate:
		return a.handleQuitImmediate(), false
	default:
		if quit := a.dispatch(nextAction); quit {
			return true, false
		}
		return false, true
	}
}

// shouldStartFilter reports whether a plain printable rune should start the quick filter.
func (a *App) shouldStartFilter(event *tcell.EventKey) bool {
	if !keymap.IsPlainPrintableRune(event) {
		return false
	}
	if _, ok := a.keys.Global.Lookup(event); ok {
		// Printable keys bound in the keymap run their action instead of opening quick filter (e.g. '-' unselect group).
		return false
	}
	return !ui.IsAuxiliaryView(a.model.ViewMode) &&
		(a.model.ViewMode != ui.ViewBrowser || a.model.ActiveSubFocus != ui.SubFocusInactivePreview) &&
		!a.model.QuickFilterStartBlocked()
}

func (a *App) shouldHandleFilterKey(event *tcell.EventKey) bool {
	f := a.activeQuickFilter()
	if keymap.IsPlainPrintableRune(event) {
		if event.Rune() == ' ' {
			return true
		}
		if id, ok := a.keys.Global.Lookup(event); ok {
			return id == keymap.ActionAppLeaderMenu
		}
		return true
	}
	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyCtrlL:
		return f.Editing || f.Active
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyInsert, tcell.KeyHome, tcell.KeyEnd:
		return f.Editing || f.Active
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if event.Modifiers()&tcell.ModCtrl != 0 {
			return f.Editing || f.Active
		}
		return f.Editing || f.Query != ""
	case tcell.KeyEnter:
		if f.Editing && strings.TrimSpace(f.Query) == "" {
			return true
		}
		return f.Active && strings.TrimSpace(f.Query) != ""
	default:
		return false
	}
}

func (a *App) stripFilterFocused() bool {
	return a.model.ViewMode == ui.ViewBrowser && a.model.ActiveSubFocus == ui.SubFocusSelectionsStrip
}

func (a *App) activeQuickFilter() panel.FilterState {
	if a.stripFilterFocused() {
		return a.activePanel().StripFilter
	}
	return a.activePanel().Filter
}

func (a *App) cancelActiveQuickFilter() {
	if a.stripFilterFocused() {
		a.activePanel().CancelStripFilter(a.selectionsStripViewportRows(a.model.ActivePanel))
		return
	}
	a.activePanel().CancelFilter(a.activeViewportRows())
}

// dispatchActionLikeKeyboardShortcut runs the same effects as the bound key in normal browser mode.
// Returns true when the app should exit immediately from handleQuit.
func (a *App) dispatchActionLikeKeyboardShortcut(actionID string) bool {
	switch actionID {
	case keymap.ActionPanelDiskUsageClear:
		a.clearAllDiskUsageData()
		return false
	case keymap.ActionCopy:
		a.dialogCtrl.ActivateCopyAction()
		return false
	case keymap.ActionMove:
		a.dialogCtrl.ActivateMoveAction()
		return false
	case keymap.ActionAppQuit:
		return a.handleQuit()
	case keymap.ActionAppQuitImmediate:
		return a.handleQuitImmediate()
	case keymap.ActionAppShowHelp:
		if !a.model.HelpView.Open && !a.model.ModalDialogOpen() {
			if a.model.Menu.Open {
				a.closeMenu()
			}
			if a.inQuickFilterUI() {
				a.cancelActiveQuickFilter()
			}
			a.openHelpDialog()
		}
		return false
	default:
		return a.dispatch(actionID)
	}
}

func (a *App) doListNav(move func()) {
	a.previewCtrl.EnsureCarouselChildCacheBeforeListNav()
	a.previewCtrl.BeginCarouselPreviewNavCoalesce()
	move()
	a.armPanelSyncFollowNavCoalesceAfterListNav()
	a.previewCtrl.ArmQuickViewNavCoalesceAfterListNav()
	a.previewCtrl.ArmCarouselPreviewNavCoalesceAfterListNav()
	a.armCursorNameHintNavCoalesceAfterListNav()
}

func (a *App) dispatch(actionID string) bool {
	if a.previewCtrl.TryDispatchFilePreviewFocus(actionID) {
		return false
	}
	if a.previewCtrl.TryDispatchQuickViewPreviewScroll(actionID) {
		return false
	}
	if a.tryDispatchSelectionsStrip(actionID) {
		return false
	}
	if a.jobsCtrl.TryDispatch(actionID) {
		return false
	}
	if a.tryDispatchMessages(actionID) {
		return false
	}
	if a.commandsCtrl.TryDispatch(actionID) {
		return false
	}
	if a.tryDispatchCompare(actionID) {
		return false
	}
	if a.tryDispatchDedup(actionID) {
		return false
	}
	if handled, quit := a.tryDispatchNavigation(actionID); handled {
		return quit
	}
	if a.tryDispatchSelectionActions(actionID) {
		return false
	}
	if a.tryDispatchPanelLayout(actionID) {
		return false
	}
	if a.dialogCtrl.TryDispatchFileOps(actionID) {
		return false
	}
	if a.previewCtrl.TryDispatchFileView(actionID) {
		return false
	}
	viewportRows := a.activeViewportRows()
	activePanel := a.activePanel()
	switch actionID {
	case keymap.ActionAppOpenMenu:
		a.openMenu()
	case keymap.ActionPanelFindDuplicates:
		a.openFindDuplicates()
	case keymap.ActionAppDropToShell:
		a.dropToShell()
	case keymap.ActionAppShellInsertPaths:
		a.shellInsertPaths()
	case keymap.ActionTerminalTogglePanel:
		a.toggleTerminalPanelVisible()
	case keymap.ActionTerminalFocus:
		a.toggleTerminalPanelFocus()
	case keymap.ActionPanelRefresh:
		a.reloadActive("Panel refreshed")
	case keymap.ActionPanelExternalBrowser:
		a.openPanelPathInExternalBrowser(a.model.ActivePanel)
	case keymap.ActionPanelDiskUsageScan:
		a.startDiskUsageScan()
	case keymap.ActionPanelSwitch:
		if a.model.QuickViewEnabled {
			a.switchPanel()
		} else if a.previewCtrl.FilePreviewOpen() {
			a.previewCtrl.CycleSubFocusForTabWithPreview()
		} else {
			a.switchPanel()
		}
	case keymap.ActionPanelFocusSelections:
		a.toggleSelectionsStripFocus()
	case keymap.ActionPanelOpenSelectionsRoot:
		a.navigateToSelectionsRoot()
	case keymap.ActionPanelMeta:
		a.metaCtrl.OpenDialog(a.model.ActivePanel)
	case keymap.ActionPanelMetaEdit:
		a.metaCtrl.EditMetaFile()
	case keymap.ActionPanelFilterOpen:
		if a.stripFilterFocused() {
			a.activePanel().OpenStripFilter(a.selectionsStripViewportRows(a.model.ActivePanel))
		} else {
			activePanel.OpenFilter(viewportRows)
		}
		a.clearTransientMessage()
	case keymap.ActionBookmarkOpen:
		a.dialogCtrl.OpenBookmarkDialog()
	case keymap.ActionBookmarkAdd:
		a.dialogCtrl.OpenAddBookmarkDialog()
	case keymap.ActionRemoteSFTPLink:
		a.openSFTPConnectDialog()
	case keymap.ActionPanelToggleSync:
		a.toggleSyncFollow()
	case keymap.ActionPanelHistoryDialog:
		// Keyboard/menu shortcut targets whichever panel is active (left vs right).
		a.openHistoryDialog(a.model.ActivePanel)
	case keymap.ActionPanelFindDialog:
		a.findCtrl.OpenDialog(a.model.ActivePanel)
	case keymap.ActionAppUserMenu:
		a.openUserMenu()
	case keymap.ActionAppLeaderMenu:
		a.toggleBuiltinLeaderMenu()
	case keymap.ActionAppCopyMenu:
		a.openCopyMenu()
	case keymap.ActionClipboardCopyFileURL,
		keymap.ActionClipboardCopyDirURL,
		keymap.ActionClipboardCopyFilename,
		keymap.ActionClipboardCopyFilenameWithoutExt:
		a.copyToClipboard(actionID)
	case keymap.ActionAppUserMenuEdit:
		a.editUserMenu()
	case keymap.ActionUIOpenTheme:
		a.openThemeDialog()
	case keymap.ActionUIOpenConfig:
		a.openConfigDialog()
	case keymap.ActionUICalibrateDebounce:
		a.openDebounceCalibrateDialog()
	case keymap.ActionMenuFileChattr:
		a.setUnsupportedMessage("Chattr")
	case keymap.ActionDevShowInfo:
		a.setTransientMessage("Example info message", ui.MessageUrgencyInfo)
	case keymap.ActionDevShowWarn:
		a.setTransientMessage("Example warn message", ui.MessageUrgencyWarn)
	case keymap.ActionDevShowError:
		a.setTransientMessage("Example error message", ui.MessageUrgencyError)
	}
	return false
}

func (a *App) handleFilterKey(event *tcell.EventKey) {
	if a.stripFilterFocused() {
		a.handleStripFilterKey(event)
		return
	}
	activePanel := a.activePanel()
	viewportRows := a.activeViewportRows()
	switch event.Key() {
	case tcell.KeyEsc:
		activePanel.CancelFilter(viewportRows)
	case tcell.KeyUp:
		activePanel.CycleFilterMatch(-1, viewportRows)
	case tcell.KeyDown:
		activePanel.CycleFilterMatch(1, viewportRows)
	case tcell.KeyHome:
		activePanel.MoveFilterCursorHome()
	case tcell.KeyEnd:
		activePanel.MoveFilterCursorEnd()
	case tcell.KeyEnter:
		if activePanel.Filter.Query != "" {
			activePanel.CancelFilter(viewportRows)
			if _, err := activePanel.Enter(viewportRows); err != nil {
				a.setErrorMessage("Enter failed", err)
			}
		} else {
			activePanel.AcceptFilter(viewportRows)
		}
	case tcell.KeyInsert:
		activePanel.ToggleSelection()
		activePanel.CycleFilterMatch(1, viewportRows)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if event.Modifiers()&tcell.ModCtrl != 0 {
			activePanel.ClearFilter(viewportRows)
		} else {
			activePanel.BackspaceFilter(viewportRows)
			// If backspace just deactivated the filter (Editing→false, Query empty),
			// arm the guard so the next key-repeat backspace does not trigger nav.parent.
			if !activePanel.Filter.Editing && activePanel.Filter.Query == "" {
				a.armNavParentBackspaceGuard()
			}
		}
	case tcell.KeyCtrlL:
		activePanel.ClearFilter(viewportRows)
	case tcell.KeyRune:
		if keymap.IsPlainPrintableRune(event) {
			activePanel.AppendFilterRune(event.Rune(), viewportRows)
		}
	}
}

func (a *App) handleStripFilterKey(event *tcell.EventKey) {
	p := a.activePanel()
	vr := a.selectionsStripViewportRows(a.model.ActivePanel)
	switch event.Key() {
	case tcell.KeyEsc:
		p.CancelStripFilter(vr)
	case tcell.KeyUp:
		p.CycleStripFilterMatch(-1, vr)
	case tcell.KeyDown:
		p.CycleStripFilterMatch(1, vr)
	case tcell.KeyEnter:
		if p.StripFilter.Query != "" {
			p.CancelStripFilter(vr)
			a.navigateFromSelectionsStrip()
		} else {
			p.AcceptStripFilter(vr)
		}
	case tcell.KeyInsert:
		p.ToggleOrRemoveStripSelection()
		if p.SelectionsStripCount() == 0 {
			p.CancelStripFilter(0)
			a.model.ActiveSubFocus = ui.SubFocusFileList
			return
		}
		p.CycleStripFilterMatch(1, vr)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if event.Modifiers()&tcell.ModCtrl != 0 {
			p.ClearStripFilter(vr)
		} else {
			p.BackspaceStripFilter(vr)
		}
	case tcell.KeyCtrlL:
		p.ClearStripFilter(vr)
	case tcell.KeyRune:
		if keymap.IsPlainPrintableRune(event) {
			p.AppendStripFilterRune(event.Rune(), vr)
		}
	}
}

func (a *App) inQuickFilterUI() bool {
	f := a.activeQuickFilter()
	return f.Active || f.Editing
}

func (a *App) handleSelectionsStripKey(event *tcell.EventKey) bool {
	p := a.activePanel()
	vr := a.selectionsStripViewportRows(a.model.ActivePanel)
	switch event.Key() {
	case tcell.KeyUp:
		p.MoveSelectionsStrip(-1, vr)
		return true
	case tcell.KeyDown:
		p.MoveSelectionsStrip(1, vr)
		return true
	case tcell.KeyPgUp:
		step := vr
		if step < 1 {
			step = 1
		}
		p.MoveSelectionsStrip(-step, vr)
		return true
	case tcell.KeyPgDn:
		step := vr
		if step < 1 {
			step = 1
		}
		p.MoveSelectionsStrip(step, vr)
		return true
	case tcell.KeyHome:
		p.SelectionsStripTop(vr)
		return true
	case tcell.KeyEnd:
		p.SelectionsStripBottom(vr)
		return true
	case tcell.KeyEnter:
		a.navigateFromSelectionsStrip()
		return true
	case tcell.KeyInsert, tcell.KeyDelete:
		p.ToggleOrRemoveStripSelection()
		if p.SelectionsStripCount() == 0 {
			p.CancelStripFilter(0)
			a.model.ActiveSubFocus = ui.SubFocusFileList
		}
		return true
	default:
		return false
	}
}
