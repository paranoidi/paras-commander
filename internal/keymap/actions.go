package keymap

// Action identifiers match docs/keybindings.md (single-stroke bindings only in v1).
const (
	ActionAppQuit          = "app.quit"
	ActionAppQuitImmediate = "app.quit-immediate"
	ActionAppOpenMenu      = "app.open-menu"
	ActionAppShowHelp      = "app.show-help"
	ActionAppUserMenu      = "app.user-menu"
	ActionAppUserMenuEdit  = "app.user-menu-edit"
	ActionAppDropToShell   = "app.drop-to-shell"
	// ActionAppShellInsertPaths puts the selected/focused paths on the persistent
	// subshell's command line and enters the shell.
	ActionAppShellInsertPaths = "app.shell-insert-paths"

	// ActionTerminalTogglePanel shows/hides the embedded terminal panel strip
	// (does not change focus).
	ActionTerminalTogglePanel = "terminal.toggle-panel"
	// ActionTerminalFocus toggles keyboard focus into/out of the terminal panel,
	// opening it first if it is hidden.
	ActionTerminalFocus = "terminal.focus"
	// ActionTerminalGrow / ActionTerminalShrink resize the terminal panel while it has focus
	// ([terminal] context only).
	ActionTerminalGrow   = "terminal.grow"
	ActionTerminalShrink = "terminal.shrink"

	ActionPanelSwitch                 = "panel.switch"
	ActionNavUp                       = "nav.up"
	ActionNavDown                     = "nav.down"
	ActionNavPageUp                   = "nav.page-up"
	ActionNavPageDown                 = "nav.page-down"
	ActionNavTop                      = "nav.top"
	ActionNavBottom                   = "nav.bottom"
	ActionNavOpen                     = "nav.open"
	ActionNavParent                   = "nav.parent"
	ActionNavHome                     = "nav.home"
	ActionNavForward                  = "nav.forward"
	ActionNavBackward                 = "nav.backward"
	ActionPanelHistoryDialog          = "panel.history-dialog"
	ActionPanelHistoryBothPanels      = "panel.history-both-panels"
	ActionPanelFindDialog             = "panel.find-dialog"
	ActionFindSelectAll               = "find.select-all"
	ActionFindUnselectAll             = "find.unselect-all"
	ActionFindSelectGroup             = "find.select-group"
	ActionFindUnselectGroup           = "find.unselect-group"
	ActionFindOpenInPrimary           = "find.open-primary"
	ActionFindOpenInSecondary         = "find.open-secondary"
	ActionPanelRefresh                = "panel.refresh"
	ActionPanelSelectToggle           = "panel.select-toggle"
	ActionPanelSelectGroup            = "panel.select-group"
	ActionPanelUnselectGroup          = "panel.unselect-group"
	ActionPanelInvertSelection        = "panel.invert-selection"
	ActionPanelClearSelection         = "panel.clear-selection"
	ActionPanelStashToggle            = "panel.stash-toggle"
	ActionPanelSortDialog             = "panel.sort-dialog"
	ActionPanelListingFormatDialog    = "panel.listing-format-dialog"
	ActionPanelCycleSort              = "panel.cycle-sort"
	ActionPanelCycleListingFormat     = "panel.cycle-listing-format"
	ActionPanelToggleCarousel         = "panel.toggle-carousel"
	ActionPanelToggleZoomActivePanel  = "panel.toggle-zoom-active-panel"
	ActionPanelToggleSplitOrientation = "panel.toggle-split-orientation"
	ActionPanelReverseSort            = "panel.reverse-sort"
	ActionPanelFilterOpen             = "panel.filter-open"
	ActionPanelToggleHidden           = "panel.toggle-hidden"
	ActionBookmarkOpen                = "bookmark.open"
	ActionBookmarkAdd                 = "bookmark.add"
	ActionBookmarkDelete              = "bookmark.delete"
	ActionBookmarkOpenOther           = "bookmark.open-other"
	ActionPanelDiskUsageScan          = "panel.disk-usage-scan"
	ActionPanelDiskUsageAbortAll      = "panel.disk-usage-abort-all"
	ActionPanelDiskUsageClear         = "panel.disk-usage-clear"
	ActionPanelFocusSelections        = "panel.focus-selections"
	ActionPanelOpenSelectionsRoot     = "panel.open-selections-root"
	ActionPanelToggleHideInactive     = "panel.toggle-hide-inactive"
	ActionPanelExternalBrowser        = "panel.external-browser"
	ActionPanelOpenDirInOther         = "panel.open-dir-in-other"
	ActionPanelOpenActivePathInOther  = "panel.open-active-path-in-other"
	ActionPanelToggleSync             = "panel.toggle-sync"
	ActionPanelMeta                   = "panel.meta"
	ActionPanelMetaEdit               = "panel.meta-edit"
	ActionPanelComparePanels          = "panel.compare-panels"
	ActionPanelFindDuplicates         = "panel.find-duplicates"

	// Compare view
	ActionCompareClose       = "compare.close"
	ActionCompareCycleFilter = "compare.cycle-filter"
	ActionCompareResetFilter = "compare.reset-filter"
	ActionCompareRefresh     = "compare.refresh"
	ActionCompareMerge       = "compare.merge"

	// Dedup (find-duplicates) view
	ActionDedupClose       = "dedup.close"
	ActionDedupToggleSort  = "dedup.toggle-sort"
	ActionDedupToggleEmpty = "dedup.toggle-empty"
	ActionDedupToggleNode  = "dedup.toggle-node"
	ActionDedupCollapse    = "dedup.collapse"
	ActionDedupToggleTree  = "dedup.toggle-tree"
	ActionDedupCollapseAll = "dedup.collapse-all"
	ActionDedupExpandAll   = "dedup.expand-all"
	ActionDedupPrevDir     = "dedup.prev-dir"
	ActionDedupNextDir     = "dedup.next-dir"
	ActionDedupMarkKeep    = "dedup.mark-keep"
	ActionDedupCompare     = "dedup.compare"

	// Dialog actions
	ActionDialogConfirm = "ui.confirm"
	ActionDialogCancel  = "ui.cancel"
	ActionDialogNext    = "ui.next-field"
	ActionDialogPrev    = "ui.prev-field"

	// File operations
	ActionFileRename = "file.rename"
	// ActionFileRenameOpenSanitize / ActionFileRenameOpenSlugify are bound via
	// [dialog.rename], not [main].
	ActionFileRenameOpenSanitize = "file.rename.open-sanitize"
	ActionFileRenameOpenSlugify  = "file.rename.open-slugify"
	ActionFileRenameOpenEncoding = "file.rename.open-encoding"
	ActionFileDelete             = "file.delete"
	ActionFileMkdir              = "file.mkdir"
	ActionFileMkdirOpenInOther   = "file.mkdir-open-in-other"
	// ActionFileMkdirExtractCommonName is bound via [dialog.mkdir], not [main].
	ActionFileMkdirExtractCommonName   = "file.mkdir.extract-common-name"
	ActionFileChmod                    = "file.chmod"
	ActionFileChown                    = "file.chown"
	ActionFileSymlink                  = "file.symlink"
	ActionFileHardlink                 = "file.hardlink"
	ActionFileExtract                  = "file.extract"
	ActionFileView                     = "file.view"
	ActionFileViewThemePicker          = "file.view.theme-picker"
	ActionFileViewDiffNextHunk         = "file.view.diff-next-hunk"
	ActionFileViewDiffPrevHunk         = "file.view.diff-prev-hunk"
	ActionFileQuickView                = "file.quick-view"
	ActionFileQuickViewPreviewPageUp   = "file.quick-view.preview-page-up"
	ActionFileQuickViewPreviewPageDown = "file.quick-view.preview-page-down"
	ActionFileEdit                     = "file.edit"
	ActionFileFlatten                  = "file.flatten"
	// ActionFlattenDestinationActive / ActionFlattenDestinationInactive are bound via
	// [dialog.flatten], not [main].
	ActionFlattenDestinationActive   = "flatten.destination-active"
	ActionFlattenDestinationInactive = "flatten.destination-inactive"

	// Copy/Move
	ActionCopy          = "file.copy"
	ActionFileDuplicate = "file.duplicate"
	ActionMove          = "file.move"

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
	ActionCommandsOpen      = "commands.open"
	ActionCommandsClose     = "commands.close"
	ActionCommandsTerminate = "commands.terminate"
	ActionCommandsKill      = "commands.kill"
	ActionFileRunForEach    = "file.run-for-each"

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

	ActionUIOpenTheme         = "ui.open-theme"
	ActionUIOpenConfig        = "ui.open-config"
	ActionUICalibrateDebounce = "ui.calibrate-debounce"

	// ActionFindSelectAll marks all ranked find-dialog results and is bound via
	// [dialog.find], not [main].
	// ActionDialogInputRestoreDefault restores a focused dialog input field's suggested default
	// (Prefill) and is bound via [dialog.input], not [main].
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
	ActionMenuFileChattr = "menu.file.chattr"
)

// Dev menu actions (menu routing only; enabled with pc -dev).
const (
	ActionDevShowInfo  = "dev.show-info"
	ActionDevShowWarn  = "dev.show-warn"
	ActionDevShowError = "dev.show-error"
)

// KnownActions lists action IDs accepted in keybindings.toml for the current app.
var KnownActions = map[string]struct{}{
	ActionAppQuit:             {},
	ActionAppQuitImmediate:    {},
	ActionAppOpenMenu:         {},
	ActionAppShowHelp:         {},
	ActionAppUserMenu:         {},
	ActionAppUserMenuEdit:     {},
	ActionAppDropToShell:      {},
	ActionAppShellInsertPaths: {},

	ActionTerminalTogglePanel: {},
	ActionTerminalFocus:       {},
	ActionTerminalGrow:        {},
	ActionTerminalShrink:      {},

	ActionPanelSwitch:                 {},
	ActionNavUp:                       {},
	ActionNavDown:                     {},
	ActionNavPageUp:                   {},
	ActionNavPageDown:                 {},
	ActionNavTop:                      {},
	ActionNavBottom:                   {},
	ActionNavOpen:                     {},
	ActionNavParent:                   {},
	ActionNavHome:                     {},
	ActionNavForward:                  {},
	ActionNavBackward:                 {},
	ActionPanelHistoryDialog:          {},
	ActionPanelHistoryBothPanels:      {},
	ActionPanelFindDialog:             {},
	ActionFindSelectAll:               {},
	ActionFindUnselectAll:             {},
	ActionFindSelectGroup:             {},
	ActionFindUnselectGroup:           {},
	ActionFindOpenInPrimary:           {},
	ActionFindOpenInSecondary:         {},
	ActionPanelRefresh:                {},
	ActionPanelSelectToggle:           {},
	ActionPanelSelectGroup:            {},
	ActionPanelUnselectGroup:          {},
	ActionPanelInvertSelection:        {},
	ActionPanelClearSelection:         {},
	ActionPanelStashToggle:            {},
	ActionPanelSortDialog:             {},
	ActionPanelListingFormatDialog:    {},
	ActionPanelCycleSort:              {},
	ActionPanelCycleListingFormat:     {},
	ActionPanelToggleCarousel:         {},
	ActionPanelToggleZoomActivePanel:  {},
	ActionPanelToggleSplitOrientation: {},
	ActionPanelReverseSort:            {},
	ActionPanelFilterOpen:             {},
	ActionPanelToggleHidden:           {},
	ActionBookmarkOpen:                {},
	ActionBookmarkAdd:                 {},
	ActionBookmarkDelete:              {},
	ActionBookmarkOpenOther:           {},
	ActionPanelDiskUsageScan:          {},
	ActionPanelDiskUsageAbortAll:      {},
	ActionPanelDiskUsageClear:         {},
	ActionPanelFocusSelections:        {},
	ActionPanelOpenSelectionsRoot:     {},
	ActionPanelToggleHideInactive:     {},
	ActionPanelExternalBrowser:        {},
	ActionPanelOpenDirInOther:         {},
	ActionPanelOpenActivePathInOther:  {},
	ActionPanelToggleSync:             {},
	ActionPanelMeta:                   {},
	ActionPanelMetaEdit:               {},
	ActionPanelComparePanels:          {},
	ActionPanelFindDuplicates:         {},

	ActionCompareClose:       {},
	ActionCompareCycleFilter: {},
	ActionCompareResetFilter: {},
	ActionCompareRefresh:     {},
	ActionCompareMerge:       {},

	ActionDedupClose:       {},
	ActionDedupToggleSort:  {},
	ActionDedupToggleEmpty: {},
	ActionDedupToggleNode:  {},
	ActionDedupCollapse:    {},
	ActionDedupToggleTree:  {},
	ActionDedupCollapseAll: {},
	ActionDedupExpandAll:   {},
	ActionDedupPrevDir:     {},
	ActionDedupNextDir:     {},
	ActionDedupMarkKeep:    {},
	ActionDedupCompare:     {},

	ActionDialogConfirm: {},
	ActionDialogCancel:  {},
	ActionDialogNext:    {},
	ActionDialogPrev:    {},

	ActionFileRename:                   {},
	ActionFileRenameOpenSanitize:       {},
	ActionFileRenameOpenSlugify:        {},
	ActionFileRenameOpenEncoding:       {},
	ActionFileDelete:                   {},
	ActionFileMkdir:                    {},
	ActionFileMkdirOpenInOther:         {},
	ActionFileMkdirExtractCommonName:   {},
	ActionFileChmod:                    {},
	ActionFileChown:                    {},
	ActionFileSymlink:                  {},
	ActionFileHardlink:                 {},
	ActionFileExtract:                  {},
	ActionFileView:                     {},
	ActionFileViewThemePicker:          {},
	ActionFileViewDiffNextHunk:         {},
	ActionFileViewDiffPrevHunk:         {},
	ActionFileQuickView:                {},
	ActionFileQuickViewPreviewPageUp:   {},
	ActionFileQuickViewPreviewPageDown: {},
	ActionFileEdit:                     {},
	ActionFileFlatten:                  {},
	ActionFlattenDestinationActive:     {},
	ActionFlattenDestinationInactive:   {},

	ActionCopy:          {},
	ActionFileDuplicate: {},
	ActionMove:          {},

	ActionRemoteSFTPLink: {},

	ActionMenuFileChattr: {},

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

	ActionCommandsOpen:      {},
	ActionCommandsClose:     {},
	ActionCommandsTerminate: {},
	ActionCommandsKill:      {},
	ActionFileRunForEach:    {},

	ActionMessagesOpen:  {},
	ActionMessagesClose: {},
	ActionMessagesClear: {},

	ActionUIOpenTheme:         {},
	ActionUIOpenConfig:        {},
	ActionUICalibrateDebounce: {},

	ActionDialogInputRestoreDefault: {},

	ActionDialogInputKillWordBackward: {},
	ActionDialogInputBackwardWord:     {},
	ActionDialogInputForwardWord:      {},
}
