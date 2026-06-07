package keymap

// Action identifiers match docs/keybindings.md (single-stroke bindings only in v1).
const (
	ActionAppQuit          = "app.quit"
	ActionAppQuitImmediate = "app.quit-immediate"
	ActionAppOpenMenu      = "app.open-menu"
	ActionAppShowHelp      = "app.show-help"
	ActionAppUserMenu      = "app.user-menu"
	ActionAppUserMenuEdit  = "app.user-menu-edit"

	ActionPanelSwitch                = "panel.switch"
	ActionNavUp                      = "nav.up"
	ActionNavDown                    = "nav.down"
	ActionNavPageUp                  = "nav.page-up"
	ActionNavPageDown                = "nav.page-down"
	ActionNavTop                     = "nav.top"
	ActionNavBottom                  = "nav.bottom"
	ActionNavOpen                    = "nav.open"
	ActionNavParent                  = "nav.parent"
	ActionNavHome                    = "nav.home"
	ActionNavForward                 = "nav.forward"
	ActionNavBackward                = "nav.backward"
	ActionPanelHistoryDialog         = "panel.history-dialog"
	ActionPanelFindDialog            = "panel.find-dialog"
	ActionFindSelectAll              = "find.select-all"
	ActionFindUnselectAll            = "find.unselect-all"
	ActionFindSelectGroup            = "find.select-group"
	ActionFindUnselectGroup          = "find.unselect-group"
	ActionPanelRefresh               = "panel.refresh"
	ActionPanelSelectToggle          = "panel.select-toggle"
	ActionPanelSelectGroup           = "panel.select-group"
	ActionPanelUnselectGroup         = "panel.unselect-group"
	ActionPanelInvertSelection       = "panel.invert-selection"
	ActionPanelClearSelection        = "panel.clear-selection"
	ActionPanelStashToggle           = "panel.stash-toggle"
	ActionPanelSortDialog            = "panel.sort-dialog"
	ActionPanelListingFormatDialog   = "panel.listing-format-dialog"
	ActionPanelCycleSort             = "panel.cycle-sort"
	ActionPanelCycleListingFormat    = "panel.cycle-listing-format"
	ActionPanelToggleCarousel        = "panel.toggle-carousel"
	ActionPanelToggleZoomActivePanel = "panel.toggle-zoom-active-panel"
	ActionPanelReverseSort           = "panel.reverse-sort"
	ActionPanelFilterOpen            = "panel.filter-open"
	ActionPanelToggleHidden          = "panel.toggle-hidden"
	ActionBookmarkOpen               = "bookmark.open"
	ActionBookmarkAdd                = "bookmark.add"
	ActionBookmarkDelete             = "bookmark.delete"
	ActionPanelDiskUsageScan         = "panel.disk-usage-scan"
	ActionPanelDiskUsageAbortAll     = "panel.disk-usage-abort-all"
	ActionPanelDiskUsageClear        = "panel.disk-usage-clear"
	ActionPanelFocusSelections       = "panel.focus-selections"
	ActionPanelToggleHideInactive    = "panel.toggle-hide-inactive"
	ActionPanelExternalBrowser       = "panel.external-browser"
	ActionPanelOpenDirInOther        = "panel.open-dir-in-other"
	ActionPanelOpenActivePathInOther = "panel.open-active-path-in-other"
	ActionPanelToggleSync            = "panel.toggle-sync"
	ActionPanelMeta                  = "panel.meta"
	ActionPanelMetaEdit              = "panel.meta-edit"

	// Dialog actions
	ActionDialogConfirm = "ui.confirm"
	ActionDialogCancel  = "ui.cancel"
	ActionDialogNext    = "ui.next-field"
	ActionDialogPrev    = "ui.prev-field"

	// File operations
	ActionFileRename = "file.rename"
	// ActionFileRenameOpenSanitize / ActionFileRenameOpenSlugify are bound via
	// [rename_dialog_action_keys], not [action_keys].
	ActionFileRenameOpenSanitize = "file.rename.open-sanitize"
	ActionFileRenameOpenSlugify  = "file.rename.open-slugify"
	ActionFileDelete             = "file.delete"
	ActionFileMkdir              = "file.mkdir"
	ActionFileMkdirOpenInOther   = "file.mkdir-open-in-other"
	ActionFileChmod              = "file.chmod"
	ActionFileChown              = "file.chown"
	ActionFileSymlink            = "file.symlink"
	ActionFileHardlink           = "file.hardlink"
	ActionFileExtract            = "file.extract"
	ActionFileView               = "file.view"
	ActionFileQuickView          = "file.quick-view"
	ActionFileEdit               = "file.edit"
	ActionFileFlatten            = "file.flatten"

	// Copy/Move
	ActionCopy         = "file.copy"
	ActionFileCopyHere = "file.copy-here"
	ActionMove         = "file.move"

	// Remote
	ActionRemoteSFTPLink = "remote.sftp-link"

	// Jobs dialog
	ActionJobsOpen          = "jobs.open"
	ActionJobsClose         = "jobs.close"
	ActionJobsNext          = "jobs.next"
	ActionJobsPrev          = "jobs.prev"
	ActionJobsDetails       = "jobs.details"
	ActionJobsClearFinished = "jobs.clear-finished"
	ActionJobsCancel        = "jobs.cancel"
	ActionJobsPause         = "jobs.pause"
	ActionJobsResume        = "jobs.resume"
	ActionJobsQueueUp       = "jobs.queue-up"
	ActionJobsQueueDown     = "jobs.queue-down"
	ActionJobsAnswerBlocker = "jobs.answer-blocker"

	// Commands screen + external command execution
	ActionCommandsOpen   = "commands.open"
	ActionCommandsClose  = "commands.close"
	ActionFileRunForEach = "file.run-for-each"

	// Messages view (status / toast log)
	ActionMessagesOpen  = "messages.open"
	ActionMessagesClose = "messages.close"
	ActionMessagesClear = "messages.clear"

	// UI dialog controls (handled internally, not in keybindings.toml)
	ActionUIConfirm   = "ui.confirm"
	ActionUICancel    = "ui.cancel"
	ActionUINextField = "ui.next-field"
	ActionUIPrevField = "ui.prev-field"
	ActionUILeft      = "ui.left"
	ActionUIRight     = "ui.right"
	ActionUIActivate  = "ui.activate"

	ActionUIOpenTheme  = "ui.open-theme"
	ActionUIOpenConfig = "ui.open-config"

	// ActionFindSelectAll marks all ranked find-dialog results and is bound via
	// [find_dialog_action_keys], not [action_keys].
	// ActionDialogInputRestoreDefault restores a focused dialog input field's suggested default
	// (Prefill) and is bound via [dialog_input_action_keys], not [action_keys].
	ActionDialogInputRestoreDefault = "ui.input.restore-default"
	// ActionDialogInputKillWordBackward deletes back to the previous word boundary (readline C-w).
	ActionDialogInputKillWordBackward = "ui.input.kill-word-backward"
	// ActionDialogInputBackwardWord moves the cursor to the previous word boundary (readline M-b).
	ActionDialogInputBackwardWord = "ui.input.backward-word"
	// ActionDialogInputForwardWord moves the cursor past the next word (readline M-f).
	ActionDialogInputForwardWord = "ui.input.forward-word"
)

// Menu routing identifiers for File pulldown entries (bindable in keybindings.toml).
const (
	ActionMenuFileViewPath        = "menu.file.view-path"
	ActionMenuFileRelativeSymlink = "menu.file.relative-symlink"
	ActionMenuFileEditSymlink     = "menu.file.edit-symlink"
	ActionMenuFileChattr          = "menu.file.chattr"
)

// Dev menu actions (menu routing only; enabled with pc -dev).
const (
	ActionDevShowInfo  = "dev.show-info"
	ActionDevShowWarn  = "dev.show-warn"
	ActionDevShowError = "dev.show-error"
)

// KnownActions lists action IDs accepted in keybindings.toml for the current app.
var KnownActions = map[string]struct{}{
	ActionAppQuit:          {},
	ActionAppQuitImmediate: {},
	ActionAppOpenMenu:      {},
	ActionAppShowHelp:      {},
	ActionAppUserMenu:      {},
	ActionAppUserMenuEdit:  {},

	ActionPanelSwitch:                {},
	ActionNavUp:                      {},
	ActionNavDown:                    {},
	ActionNavPageUp:                  {},
	ActionNavPageDown:                {},
	ActionNavTop:                     {},
	ActionNavBottom:                  {},
	ActionNavOpen:                    {},
	ActionNavParent:                  {},
	ActionNavHome:                    {},
	ActionNavForward:                 {},
	ActionNavBackward:                {},
	ActionPanelHistoryDialog:         {},
	ActionPanelFindDialog:            {},
	ActionFindSelectAll:              {},
	ActionFindUnselectAll:            {},
	ActionFindSelectGroup:            {},
	ActionFindUnselectGroup:          {},
	ActionPanelRefresh:               {},
	ActionPanelSelectToggle:          {},
	ActionPanelSelectGroup:           {},
	ActionPanelUnselectGroup:         {},
	ActionPanelInvertSelection:       {},
	ActionPanelClearSelection:        {},
	ActionPanelStashToggle:           {},
	ActionPanelSortDialog:            {},
	ActionPanelListingFormatDialog:   {},
	ActionPanelCycleSort:             {},
	ActionPanelCycleListingFormat:    {},
	ActionPanelToggleCarousel:        {},
	ActionPanelToggleZoomActivePanel: {},
	ActionPanelReverseSort:           {},
	ActionPanelFilterOpen:            {},
	ActionPanelToggleHidden:          {},
	ActionBookmarkOpen:               {},
	ActionBookmarkAdd:                {},
	ActionBookmarkDelete:             {},
	ActionPanelDiskUsageScan:         {},
	ActionPanelDiskUsageAbortAll:     {},
	ActionPanelDiskUsageClear:        {},
	ActionPanelFocusSelections:       {},
	ActionPanelToggleHideInactive:    {},
	ActionPanelExternalBrowser:       {},
	ActionPanelOpenDirInOther:        {},
	ActionPanelOpenActivePathInOther: {},
	ActionPanelToggleSync:            {},
	ActionPanelMeta:                  {},
	ActionPanelMetaEdit:              {},

	ActionDialogConfirm: {},
	ActionDialogCancel:  {},
	ActionDialogNext:    {},
	ActionDialogPrev:    {},

	ActionFileRename:             {},
	ActionFileRenameOpenSanitize: {},
	ActionFileRenameOpenSlugify:  {},
	ActionFileDelete:             {},
	ActionFileMkdir:              {},
	ActionFileMkdirOpenInOther:   {},
	ActionFileChmod:              {},
	ActionFileChown:              {},
	ActionFileSymlink:            {},
	ActionFileHardlink:           {},
	ActionFileExtract:            {},
	ActionFileView:               {},
	ActionFileQuickView:          {},
	ActionFileEdit:               {},
	ActionFileFlatten:            {},

	ActionCopy:         {},
	ActionFileCopyHere: {},
	ActionMove:         {},

	ActionRemoteSFTPLink: {},

	ActionMenuFileViewPath:        {},
	ActionMenuFileRelativeSymlink: {},
	ActionMenuFileEditSymlink:     {},
	ActionMenuFileChattr:          {},

	ActionJobsOpen:          {},
	ActionJobsClose:         {},
	ActionJobsNext:          {},
	ActionJobsPrev:          {},
	ActionJobsDetails:       {},
	ActionJobsClearFinished: {},
	ActionJobsCancel:        {},
	ActionJobsPause:         {},
	ActionJobsResume:        {},
	ActionJobsQueueUp:       {},
	ActionJobsQueueDown:     {},
	ActionJobsAnswerBlocker: {},

	ActionCommandsOpen:   {},
	ActionCommandsClose:  {},
	ActionFileRunForEach: {},

	ActionMessagesOpen:  {},
	ActionMessagesClose: {},
	ActionMessagesClear: {},

	ActionUIOpenTheme:  {},
	ActionUIOpenConfig: {},

	ActionDialogInputRestoreDefault: {},

	ActionDialogInputKillWordBackward: {},
	ActionDialogInputBackwardWord:     {},
	ActionDialogInputForwardWord:      {},
}
