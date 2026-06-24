package app

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui"
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
	InputModeFilePreviewView
	InputModePathPicker
	InputModeHistoryDialog
	InputModeFindDialog
	InputModeMetaDialog
	InputModeUserMenu
	InputModeHelpView
	InputModeHostKeyDialog
	InputModeSFTPConnectDialog
)

func (a *App) inputMode() InputMode {
	switch {
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
	case a.model.UserMenu.Open:
		return InputModeUserMenu
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
	case a.model.ViewMode == ui.ViewFilePreview &&
		!a.model.AuxiliaryViewDialogKeysBlocked() &&
		!a.inQuickFilterUI():
		return InputModeFilePreviewView
	case a.model.ViewMode == ui.ViewCommands &&
		!a.model.AuxiliaryViewDialogKeysBlocked() &&
		!a.inQuickFilterUI():
		return InputModeCommandsView
	case a.model.ViewMode == ui.ViewMessages &&
		!a.model.AuxiliaryViewDialogKeysBlocked() &&
		!a.inQuickFilterUI():
		return InputModeMessagesView
	case a.model.ViewMode == ui.ViewJobs &&
		!a.model.AuxiliaryViewDialogKeysBlocked() &&
		!a.inQuickFilterUI():
		return InputModeJobsView
	case a.model.TransferDialog.Open, a.model.FlattenDialog.Open, a.model.ConflictDialog.Open, a.model.QuitConfirm.Open, a.model.StashRestoreDialog.Open:
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
	if a.model.MessageDialog.Open {
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
	if a.model.UserMenu.Open {
		return footerWithEscClose([]menu.FunctionKey{
			menu.FunctionKeyEditConfig,
			{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
		})
	}
	if a.model.FindDialog.Open {
		rest := []menu.FunctionKey{{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"}}
		rest = append(findDialogOverlayFooterKeys(a.keysFindDialog), rest...)
		return footerWithEscClose(rest)
	}
	if a.model.HistoryDialog.Open {
		rest := []menu.FunctionKey{{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"}}
		rest = append(historyDialogOverlayFooterKeys(a.keysHistoryDialog, a.model.HistoryDialog.BothPanels), rest...)
		return footerWithEscClose(rest)
	}
	if a.model.PathPicker.Open || a.model.MetaDialog.Open {
		rest := []menu.FunctionKey{{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"}}
		if a.model.MetaDialog.Open {
			rest = append([]menu.FunctionKey{menu.FunctionKeyEditConfig}, rest...)
		}
		if a.bookmarkDialogDeleteFooterEligible() {
			if lbl := a.keysBookmarkDialog.MenuBindingLabel(keymap.ActionBookmarkDelete); lbl != "" {
				rest = append([]menu.FunctionKey{{Key: tcell.KeyF8, KeyLabel: lbl, Hint: "Delete"}}, rest...)
			}
		}
		return footerWithEscClose(rest)
	}
	if a.model.PrimaryModal() != ui.PrimaryModalNone ||
		a.model.SortDialog.Open || a.model.ListingFormatDialog.Open || a.model.ConfigDialog.Open || a.model.GroupSelect.Open || a.model.FileDialog.Open || a.model.SFTPConnectDialog.Open || a.model.PathPicker.Open || a.model.HistoryDialog.Open || a.model.FindDialog.Open || a.model.MetaDialog.Open || a.model.UserMenu.Open {
		rest := []menu.FunctionKey{{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"}}
		if hints := flattenDialogOverlayFooterKeys(a, a.keysFlattenDialog); len(hints) > 0 {
			rest = append(hints, rest...)
		}
		if a.pathPickerHostFooterEligible() {
			if lbl := a.keys.MenuBindingLabel(keymap.ActionBookmarkOpen); lbl != "" {
				rest = append([]menu.FunctionKey{{KeyLabel: lbl, Hint: "Bookmarks"}}, rest...)
			}
		}
		if a.mkdirDialogExtractFooterEligible() {
			if lbl := a.keysMkdirDialog.MenuBindingLabel(keymap.ActionFileMkdirExtractCommonName); lbl != "" {
				rest = append([]menu.FunctionKey{{Key: tcell.KeyF7, KeyLabel: lbl, Hint: "Extract common name"}}, rest...)
			}
		}
		if a.dialogInputRestoreFooterEligible() {
			if lbl := a.keysDialogInput.MenuBindingLabel(keymap.ActionDialogInputRestoreDefault); lbl != "" {
				rest = append([]menu.FunctionKey{{KeyLabel: lbl, Hint: "Default"}}, rest...)
			}
		}
		if a.massRenameEditorFooterEligible() {
			rest = append([]menu.FunctionKey{{Key: tcell.KeyF4, KeyLabel: "F4", Hint: "Editor"}}, rest...)
		}
		if a.renameDialogFooterEligible() {
			if a.renameEncodingFooterEligible() {
				if lbl := a.keysRenameDialog.MenuBindingLabel(keymap.ActionFileRenameOpenEncoding); lbl != "" {
					rest = append([]menu.FunctionKey{{Key: tcell.KeyF4, KeyLabel: lbl, Hint: "Encoding"}}, rest...)
				}
			}
			if lbl := a.keysRenameDialog.MenuBindingLabel(keymap.ActionFileRenameOpenSlugify); lbl != "" {
				rest = append([]menu.FunctionKey{{Key: tcell.KeyF3, KeyLabel: lbl, Hint: "Slugify"}}, rest...)
			}
			if lbl := a.keysRenameDialog.MenuBindingLabel(keymap.ActionFileRenameOpenSanitize); lbl != "" {
				rest = append([]menu.FunctionKey{{Key: tcell.KeyF2, KeyLabel: lbl, Hint: "Sanitize"}}, rest...)
			}
		}
		return footerWithEscClose(rest)
	}
	if a.model.Menu.Open {
		// Menu open: Esc closes menu / pulldown; F9 and F10 as before.
		return footerWithEscClose([]menu.FunctionKey{
			{Key: tcell.KeyF9, KeyLabel: "F9", Hint: "Menu"},
			{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
		})
	}
	if a.model.ViewMode == ui.ViewFilePreview && !a.inQuickFilterUI() {
		if a.model.FilePreviewThemePicker.Open {
			return menu.FunctionKeysFilePreviewStylePicker()
		}
		return menu.FunctionKeysFilePreviewView()
	}
	if a.model.ViewMode == ui.ViewCommands && !a.inQuickFilterUI() {
		return menu.FunctionKeysCommandsView()
	}
	if a.model.ViewMode == ui.ViewMessages && !a.inQuickFilterUI() {
		return menu.FunctionKeysMessagesView()
	}
	if a.model.ViewMode == ui.ViewJobs && !a.inQuickFilterUI() {
		return menu.FunctionKeysJobsView()
	}
	if a.model.HelpView.Open {
		return footerWithEscClose([]menu.FunctionKey{
			{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
		})
	}
	if a.model.ViewMode == ui.ViewBrowser &&
		a.model.ActiveSubFocus == ui.SubFocusSelectionsStrip &&
		!a.inQuickFilterUI() {
		return menu.FunctionKeysSelectionsStripView(a.keys.MenuBindingLabel(keymap.ActionPanelClearSelection))
	}
	return menu.FunctionKeys
}

func footerWithEscClose(rest []menu.FunctionKey) []menu.FunctionKey {
	out := make([]menu.FunctionKey, 0, len(rest)+1)
	out = append(out, menu.FooterEscClose)
	out = append(out, rest...)
	return out
}

func (a *App) prepareGlobalQuitShortcutCleanup() {
	a.clearPanelSyncFollowNavCoalesce()
	a.clearQuickViewNavCoalesce()
	a.clearCarouselPreviewNavCoalesce()
	a.clearCursorNameHintNavCoalesce()
	if a.inQuickFilterUI() {
		a.activePanel().CancelFilter(a.activeViewportRows())
	}
}

func (a *App) handleKey(event *tcell.EventKey) (quit bool, rendered bool) {
	a.deferDiskIdleSortOnUserActivity()
	// Global F10 quit - works from any mode, any dialog, any menu.
	// Shift+F10 defaults to app.quit-immediate and must not fall through to plain F10.
	if event.Key() == tcell.KeyF10 {
		if event.Modifiers()&tcell.ModShift != 0 {
			if id, ok := a.keys.Lookup(event); ok && id == keymap.ActionAppQuitImmediate {
				a.prepareGlobalQuitShortcutCleanup()
				return a.handleQuitImmediate(), false
			}
		} else {
			a.prepareGlobalQuitShortcutCleanup()
			if a.model.QuitConfirm.Open {
				a.model.QuitConfirm = ui.QuitConfirmState{}
				a.stopWorker()
				return true, false
			}
			return a.handleQuit(), false
		}
	}

	if id, ok := a.keys.Lookup(event); ok && id == keymap.ActionAppQuitImmediate {
		a.prepareGlobalQuitShortcutCleanup()
		return a.handleQuitImmediate(), false
	}

	resolvedAction := a.actionFromKeyEvent(event)
	if !a.panelSyncFollowHeldListNav(resolvedAction, event) {
		a.clearPanelSyncFollowNavCoalesce()
		a.clearQuickViewNavCoalesce()
		a.clearCursorNameHintNavCoalesce()
	}
	if !a.carouselPreviewHeldListNav(resolvedAction, event) {
		a.clearCarouselPreviewNavCoalesce()
	}
	if !a.model.ModalDialogOpen() {
		if resolvedAction == keymap.ActionPanelDiskUsageAbortAll {
			a.abortAllDiskUsageScans()
			a.render()
			return false, true
		}
		if resolvedAction == keymap.ActionPanelDiskUsageClear {
			a.clearAllDiskUsageData()
			a.render()
			return false, true
		}
	}

	if resolvedAction == keymap.ActionJobsAnswerBlocker {
		if rendered := a.handleJobsAnswerBlockerKey(); rendered {
			a.render()
			return false, true
		}
		return false, false
	}

	// Global show-help (F1 by default). Closes menu or quick filter first.
	if resolvedAction == keymap.ActionAppShowHelp && !a.model.HelpView.Open {
		// Do not open help from modal dialogs.
		if a.model.ModalDialogOpen() {
			return false, false
		}
		if a.model.Menu.Open {
			a.closeMenu()
		}
		if a.inQuickFilterUI() {
			a.activePanel().CancelFilter(a.activeViewportRows())
		}
		a.openHelpDialog()
		a.render()
		return false, true
	}

	switch a.inputMode() {
	case InputModeMessageDialog:
		a.handleMessageDialogKey(event)
		a.render()
		return false, true
	case InputModePathPicker:
		a.handlePathPickerKey(event)
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
		a.handleFindDialogKey(event)
		if !wasOpen || !a.model.FindDialog.Open || (!gsWasOpen && a.model.GroupSelect.Open) {
			a.render()
		} else if !a.paintFindDialogOverlay() {
			a.render()
		}
		return false, true
	case InputModeMetaDialog:
		a.handleMetaDialogKey(event)
		a.render()
		return false, true
	case InputModeUserMenu:
		a.handleUserMenuDialogKey(event)
		a.render()
		return false, true
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
		wasDelete := a.deleteDialogOpen()
		quit := a.handleFileDialogKey(event)
		if wasDelete && a.deleteDialogOpen() {
			if !a.paintDeleteDialogOverlay() {
				a.render()
			}
		} else {
			a.render()
		}
		return quit, true
	case InputModeJobsView:
		quit := a.handleJobsViewKey(event)
		a.render()
		return quit, true
	case InputModeCommandsView:
		quit := a.handleCommandsViewKey(event)
		a.render()
		return quit, true
	case InputModeMessagesView:
		quit := a.handleMessagesViewKey(event)
		a.render()
		return quit, true
	case InputModeFilePreviewView:
		quit := a.handleFilePreviewViewKey(event)
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
		// Function keys in filter mode dismiss the filter and run the menu action.
		if _, ok := menu.FunctionKeyLabelByKey(event.Key()); ok {
			quit := a.handleQuickFilterFunctionKey(event)
			a.render()
			return quit, true
		}
		// Bound actions (same keymap as normal browser mode) dismiss the filter unless
		// the key is filter-local (typing, match cycling, Insert, etc.).
		if resolvedAction != "" && !a.quickFilterRetainsKey(event, resolvedAction) {
			a.activePanel().CancelFilter(a.activeViewportRows())
			fqQuit, fqRendered := a.finishResolvedKeyboardAction(resolvedAction)
			if fqRendered {
				a.render()
			}
			return fqQuit, fqRendered
		}
		// Filter keys (printable, navigation, etc.) update the filter.
		if a.shouldHandleFilterKey(event) {
			a.handleFilterKey(event)
			a.render()
			return false, true
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

// quickFilterRetainsKey reports keys that stay on the quick-filter input path even when
// the same chord is bound to a panel action (match cycling, Insert, query editing).
func (a *App) quickFilterRetainsKey(event *tcell.EventKey, resolvedAction string) bool {
	switch event.Key() {
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyInsert:
		return true
	}
	if isPlainPrintableRune(event) {
		return resolvedAction == ""
	}
	f := a.activePanel().Filter
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
		a.activateCopyAction()
		return false, true
	case keymap.ActionMove:
		a.activateMoveAction()
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
	if !isPlainPrintableRune(event) {
		return false
	}
	if _, ok := a.keys.Lookup(event); ok {
		// Printable keys bound in the keymap run their action instead of opening quick filter (e.g. '-' unselect group).
		return false
	}
	return !ui.IsAuxiliaryView(a.model.ViewMode) &&
		(a.model.ViewMode != ui.ViewBrowser ||
			(a.model.ActiveSubFocus != ui.SubFocusSelectionsStrip && a.model.ActiveSubFocus != ui.SubFocusInactivePreview)) &&
		!a.model.QuickFilterStartBlocked()
}

func (a *App) shouldHandleFilterKey(event *tcell.EventKey) bool {
	f := a.activePanel().Filter
	if isPlainPrintableRune(event) {
		if _, ok := a.keys.Lookup(event); ok {
			return false
		}
		return true
	}
	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyCtrlL:
		return f.Editing || f.Active
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyInsert:
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

// dispatchActionLikeKeyboardShortcut runs the same effects as the bound key in normal browser mode.
// Returns true when the app should exit immediately from handleQuit.
func (a *App) dispatchActionLikeKeyboardShortcut(actionID string) bool {
	switch actionID {
	case keymap.ActionPanelDiskUsageAbortAll:
		a.abortAllDiskUsageScans()
		return false
	case keymap.ActionPanelDiskUsageClear:
		a.clearAllDiskUsageData()
		return false
	case keymap.ActionCopy:
		a.activateCopyAction()
		return false
	case keymap.ActionMove:
		a.activateMoveAction()
		return false
	case keymap.ActionAppQuit:
		return a.handleQuit()
	case keymap.ActionAppQuitImmediate:
		return a.handleQuitImmediate()
	default:
		return a.dispatch(actionID)
	}
}

func (a *App) doListNav(move func()) {
	a.ensureCarouselChildCacheBeforeListNav()
	a.beginCarouselPreviewNavCoalesce()
	move()
	a.armPanelSyncFollowNavCoalesceAfterListNav()
	a.armQuickViewNavCoalesceAfterListNav()
	a.armCarouselPreviewNavCoalesceAfterListNav()
	a.armCursorNameHintNavCoalesceAfterListNav()
}

func (a *App) dispatch(actionID string) bool {
	if a.tryDispatchFilePreviewFocus(actionID) {
		return false
	}
	if a.tryDispatchQuickViewPreviewScroll(actionID) {
		return false
	}
	if a.tryDispatchSelectionsStrip(actionID) {
		return false
	}
	if a.tryDispatchJobs(actionID) {
		return false
	}
	if a.tryDispatchMessages(actionID) {
		return false
	}
	if a.tryDispatchCommands(actionID) {
		return false
	}
	viewportRows := a.activeViewportRows()
	activePanel := a.activePanel()
	switch actionID {
	case keymap.ActionAppOpenMenu:
		a.openMenu()
	case keymap.ActionAppDropToShell:
		a.dropToShell()
	case keymap.ActionPanelRefresh:
		a.reloadActive("Panel refreshed")
	case keymap.ActionPanelExternalBrowser:
		a.openPanelPathInExternalBrowser(a.model.ActivePanel)
	case keymap.ActionPanelDiskUsageScan:
		a.startDiskUsageScan()
	case keymap.ActionPanelSwitch:
		if a.model.QuickViewEnabled {
			a.switchPanel()
		} else if a.filePreviewOpen() {
			a.cycleSubFocusForTabWithPreview()
		} else {
			a.switchPanel()
		}
	case keymap.ActionPanelFocusSelections:
		a.toggleSelectionsStripFocus()
	case keymap.ActionPanelToggleHideInactive:
		a.toggleHideInactivePanel()
	case keymap.ActionNavUp:
		a.doListNav(func() { activePanel.Move(-1, viewportRows) })
	case keymap.ActionNavDown:
		a.doListNav(func() { activePanel.Move(1, viewportRows) })
	case keymap.ActionNavPageUp:
		a.doListNav(func() { activePanel.Page(-1, viewportRows) })
	case keymap.ActionNavPageDown:
		a.doListNav(func() { activePanel.Page(1, viewportRows) })
	case keymap.ActionNavTop:
		a.doListNav(func() { activePanel.Top(viewportRows) })
	case keymap.ActionNavBottom:
		a.doListNav(func() { activePanel.Bottom(viewportRows) })
	case keymap.ActionPanelSelectToggle:
		_, conflicts := activePanel.ToggleSelectionAndAdvance(viewportRows)
		if conflicts {
			a.setTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
		}
	case keymap.ActionPanelSelectGroup:
		a.openGroupSelect("select", "panel")
	case keymap.ActionPanelUnselectGroup:
		a.openGroupSelect("unselect", "panel")
	case keymap.ActionPanelInvertSelection:
		activePanel.InvertSelection()
		a.setTransientMessage("Selection inverted", ui.MessageUrgencyInfo)
	case keymap.ActionPanelClearSelection:
		activePanel.ClearSelection()
		a.setTransientMessage("Selection cleared", ui.MessageUrgencyInfo)
	case keymap.ActionPanelStashToggle:
		a.togglePanelSelectionStash()
	case keymap.ActionPanelSortDialog:
		a.openSortDialog()
	case keymap.ActionPanelListingFormatDialog:
		if activePanel.CarouselMode {
			a.setTransientMessage("Listing format is not available in carousel view", ui.MessageUrgencyInfo)
			break
		}
		a.openListingFormatDialog()
	case keymap.ActionPanelMeta:
		a.openMetaDialog(a.model.ActivePanel)
	case keymap.ActionPanelMetaEdit:
		a.editMetaFile()
	case keymap.ActionPanelCycleSort:
		activePanel.CycleSort(viewportRows)
		a.setTransientMessage(fmt.Sprintf("Sort: %s", activePanel.Sort.Mode.String()), ui.MessageUrgencyInfo)
	case keymap.ActionPanelCycleListingFormat:
		if activePanel.CarouselMode {
			a.setTransientMessage("Listing format is not available in carousel view", ui.MessageUrgencyInfo)
			break
		}
		activePanel.CycleListingFormat()
		a.setTransientMessage(fmt.Sprintf("Listing: %s", activePanel.ListFormat.String()), ui.MessageUrgencyInfo)
	case keymap.ActionPanelToggleCarousel:
		activePanel.CarouselMode = !activePanel.CarouselMode
		if !activePanel.CarouselMode {
			a.clearCarouselPreviewNavCoalesce()
			a.closeCarouselFilePreview()
		}
		onOff := "off"
		if activePanel.CarouselMode {
			onOff = "on"
		}
		a.setTransientMessage(fmt.Sprintf("Carousel view: %s", onOff), ui.MessageUrgencyInfo)
	case keymap.ActionPanelToggleZoomActivePanel:
		if a.filePreviewOpen() || a.model.QuickViewDisplayActive() {
			a.setTransientMessage("Zoom disabled while quick view or file view is active", ui.MessageUrgencyInfo)
			break
		}
		if activePanel.CarouselMode {
			a.setTransientMessage("Panel zoom is always on in carousel view", ui.MessageUrgencyInfo)
			break
		}
		tw, th := a.screen.Size()
		orientation := a.effectivePaneSplitOrientation()
		if orientation == ui.SplitVertical {
			if a.zoomActivePanelSuppressedByTerminalHeight(th) {
				a.setTransientMessage(fmt.Sprintf(
					"Panel zoom unavailable (terminal height ≥ %d)",
					a.config.UI.ZoomActivePanelDisabledAboveHeight,
				), ui.MessageUrgencyInfo)
				break
			}
		} else if a.zoomActivePanelSuppressedByTerminalWidth(tw) {
			a.setTransientMessage(fmt.Sprintf(
				"Panel zoom unavailable (terminal width ≥ %d)",
				a.config.UI.ZoomActivePanelDisabledAboveWidth,
			), ui.MessageUrgencyInfo)
			break
		}
		a.toggleRuntimeZoomActivePanel()
	case keymap.ActionPanelToggleSplitOrientation:
		a.togglePaneSplitOrientation()
	case keymap.ActionPanelReverseSort:
		activePanel.ToggleSortReverse(viewportRows)
		direction := "ascending"
		if activePanel.Sort.Reverse {
			direction = "descending"
		}
		a.setTransientMessage(fmt.Sprintf("Sort %s (%s)", direction, activePanel.Sort.Mode.String()), ui.MessageUrgencyInfo)
	case keymap.ActionPanelToggleHidden:
		if err := a.panelByID(a.model.ActivePanel).ToggleHidden(viewportRows); err != nil {
			a.setErrorMessage("Toggle hidden failed", err)
			return false
		}
		label := panelLabel(a.model.ActivePanel)
		a.setTransientMessage(a.paneHiddenVisibilityMessage(label, a.activePanel().ShowHidden), ui.MessageUrgencyInfo)
	case keymap.ActionPanelFilterOpen:
		activePanel.OpenFilter(viewportRows)
		a.clearTransientMessage()
	case keymap.ActionBookmarkOpen:
		a.openBookmarkDialog()
	case keymap.ActionBookmarkAdd:
		a.openAddBookmarkDialog()
	case keymap.ActionRemoteSFTPLink:
		a.openSFTPConnectDialog()
	case keymap.ActionNavOpen:
		return a.handleNavOpen(activePanel, viewportRows)
	case keymap.ActionPanelToggleSync:
		a.toggleSyncFollow()
	case keymap.ActionPanelOpenDirInOther:
		if a.model.ViewMode != ui.ViewBrowser {
			return false
		}
		entry, ok := activePanel.CurrentEntry()
		if !ok || entry.Type != localfs.EntryDirectory {
			return false
		}
		if err := a.navigatePanelToDirectory(a.inactivePanelID(), entry.Path, ""); err != nil {
			a.setErrorMessage("Open in other panel failed", err)
			return false
		}
		if a.disableSyncFollowIfEnabled() {
			a.setTransientMessage("Open in other panel — sync disabled", ui.MessageUrgencyWarn)
		}
	case keymap.ActionPanelOpenActivePathInOther:
		if a.model.ViewMode != ui.ViewBrowser {
			return false
		}
		if err := a.navigatePanelToDirectory(a.inactivePanelID(), activePanel.PathString(), ""); err != nil {
			a.setErrorMessage("Open current path in other panel failed", err)
			return false
		}
		if a.disableSyncFollowIfEnabled() {
			a.setTransientMessage("Open in other panel — sync disabled", ui.MessageUrgencyWarn)
		}
	case keymap.ActionNavParent:
		if err := activePanel.Parent(viewportRows); err != nil {
			a.setErrorMessage("Parent failed", err)
			return false
		}
	case keymap.ActionNavHome:
		if a.model.UserHomeDir == "" {
			return false
		}
		if err := a.navigatePanelToDirectory(a.model.ActivePanel, a.model.UserHomeDir, ""); err != nil {
			a.setErrorMessage("Navigate to home failed", err)
		}
	case keymap.ActionNavForward:
		if _, err := activePanel.HistoryForward(viewportRows); err != nil {
			a.setErrorMessage("Forward history failed", err)
			return false
		}
	case keymap.ActionNavBackward:
		if _, err := activePanel.HistoryBackward(viewportRows); err != nil {
			a.setErrorMessage("Backward history failed", err)
			return false
		}
	case keymap.ActionPanelHistoryDialog:
		// Keyboard/menu shortcut targets whichever panel is active (left vs right).
		a.openHistoryDialog(a.model.ActivePanel)
	case keymap.ActionPanelFindDialog:
		a.openFindDialog(a.model.ActivePanel)
		// File operation actions
	case keymap.ActionFileRename:
		a.openRenameDialog(activePanel)
	case keymap.ActionFileDelete:
		a.openDeleteDialog(activePanel)
	case keymap.ActionFileMkdir:
		a.openMkdirDialog(false)
	case keymap.ActionFileMkdirOpenInOther:
		a.openMkdirDialog(true)
	case keymap.ActionFileChmod:
		a.openChmodDialog(activePanel)
	case keymap.ActionFileChown:
		a.openChownDialog(activePanel)
	case keymap.ActionFileSymlink:
		a.openSymlinkDialog(activePanel)
	case keymap.ActionFileHardlink:
		a.openHardlinkDialog(activePanel)
	case keymap.ActionFileExtract:
		a.openExtractDialog(activePanel)
	case keymap.ActionFileFlatten:
		a.openFlattenDialog()
	case keymap.ActionCopy:
		a.activateCopyAction()
	case keymap.ActionFileCopyHere:
		a.activateCopyHereAction()
	case keymap.ActionMove:
		a.activateMoveAction()
	case keymap.ActionFileRunForEach:
		if a.model.ViewMode != ui.ViewBrowser {
			return false
		}
		a.openRunForEachDialog()
	case keymap.ActionAppUserMenu:
		a.openUserMenu()
	case keymap.ActionAppUserMenuEdit:
		a.editUserMenu()
	case keymap.ActionUIOpenTheme:
		a.openThemeDialog()
	case keymap.ActionUIOpenConfig:
		a.openConfigDialog()
	case keymap.ActionUICalibrateDebounce:
		a.openDebounceCalibrateDialog()
	case keymap.ActionFileView:
		a.openFilePreviewFullscreen()
	case keymap.ActionFileViewThemePicker:
		a.toggleFilePreviewThemePicker()
	case keymap.ActionFileViewDiffNextHunk:
		a.hunkNavigate(previewTargetInactive, 1)
	case keymap.ActionFileViewDiffPrevHunk:
		a.hunkNavigate(previewTargetInactive, -1)
	case keymap.ActionFileQuickView:
		a.handleQuickViewToggle()
	case keymap.ActionFileEdit:
		if a.model.ViewMode == ui.ViewFilePreview && a.model.FullscreenFilePreview.Open {
			a.editFullscreenPreviewFile()
		} else {
			a.editActiveFile()
		}
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
	activePanel := a.activePanel()
	viewportRows := a.activeViewportRows()
	switch event.Key() {
	case tcell.KeyEsc:
		activePanel.CancelFilter(viewportRows)
	case tcell.KeyUp:
		activePanel.CycleFilterMatch(-1, viewportRows)
	case tcell.KeyDown:
		activePanel.CycleFilterMatch(1, viewportRows)
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
		if isPlainPrintableRune(event) {
			activePanel.AppendFilterRune(event.Rune(), viewportRows)
		}
	}
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
			a.model.ActiveSubFocus = ui.SubFocusFileList
		}
		return true
	default:
		return false
	}
}

func isPlainPrintableRune(event *tcell.EventKey) bool {
	return event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModNone && unicode.IsPrint(event.Rune())
}

// isDialogInputRune reports a printable rune suitable for dialog text fields.
// Shifted punctuation (e.g. Shift+4 → '$') often arrives with ModShift; Alt/Ctrl are rejected.
func isDialogInputRune(event *tcell.EventKey) bool {
	if event.Key() != tcell.KeyRune || !unicode.IsPrint(event.Rune()) {
		return false
	}
	mod := event.Modifiers()
	return mod == tcell.ModNone || mod == tcell.ModShift
}

func (a *App) inQuickFilterUI() bool {
	filter := a.activePanel().Filter
	return filter.Active || filter.Editing
}
