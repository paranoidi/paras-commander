package keymap

// Action identifiers match docs/keybindings.md (single-stroke bindings only in v1).
const (
	ActionAppQuit          = "app.quit"
	ActionAppQuitImmediate = "app.quit-immediate"
	ActionAppOpenMenu      = "app.open-menu"
	ActionAppShowHelp      = "app.show-help"
	ActionAppUserMenu      = "app.user-menu"
	ActionAppUserMenuEdit  = "app.user-menu-edit"
	ActionAppLeaderMenu    = "app.leader-menu"
	ActionAppCopyMenu      = "app.copy-menu"
	ActionAppDropToShell   = "app.drop-to-shell"

	ActionClipboardCopyFileURL            = "clipboard.copy-file-url"
	ActionClipboardCopyDirURL             = "clipboard.copy-dir-url"
	ActionClipboardCopyFilename           = "clipboard.copy-filename"
	ActionClipboardCopyFilenameWithoutExt = "clipboard.copy-filename-without-ext"
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

	ActionPanelSwitch            = "panel.switch"
	ActionPanelViMotionToggle    = "panel.vi-motion-toggle"
	ActionNavUp                  = "nav.up"
	ActionNavDown                = "nav.down"
	ActionNavPageUp              = "nav.page-up"
	ActionNavPageDown            = "nav.page-down"
	ActionNavTop                 = "nav.top"
	ActionNavBottom              = "nav.bottom"
	ActionNavOpen                = "nav.open"
	ActionNavParent              = "nav.parent"
	ActionNavHome                = "nav.home"
	ActionNavForward             = "nav.forward"
	ActionNavBackward            = "nav.backward"
	ActionPanelHistoryDialog     = "panel.history-dialog"
	ActionPanelGitFilterMenu     = "panel.git-filter-menu"
	ActionPanelHistoryBothPanels = "panel.history-both-panels"
	// ActionHelpTextEditKeys is bound via [dialog.help], not [main].
	ActionHelpTextEditKeys            = "help.text-edit-keys"
	ActionPanelFindDialog             = "panel.find-dialog"
	ActionFindView                    = "find.view"
	ActionFindSelectAll               = "find.select-all"
	ActionFindUnselectAll             = "find.unselect-all"
	ActionFindSelectGroup             = "find.select-group"
	ActionFindUnselectGroup           = "find.unselect-group"
	ActionFindSelectParentDirs        = "find.select-parent-dirs"
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
	ActionPanelToggleTree             = "panel.toggle-tree"
	ActionPanelTreeExpand             = "panel.tree-expand"
	ActionPanelTreeCollapse           = "panel.tree-collapse"
	ActionPanelTreeCollapseAll        = "panel.tree-collapse-all"
	ActionPanelTreeCollapseAllFull    = "panel.tree-collapse-all-full"
	ActionPanelTreeExpandAllShallow   = "panel.tree-expand-all-shallow"
	ActionPanelTreeExpandAllFull      = "panel.tree-expand-all-full"
	ActionPanelTreePrevSiblingDir     = "panel.tree-prev-sibling-dir"
	ActionPanelTreeNextSiblingDir     = "panel.tree-next-sibling-dir"
	ActionPanelToggleZoomActivePanel  = "panel.toggle-zoom-active-panel"
	ActionPanelToggleSplitOrientation = "panel.toggle-split-orientation"
	ActionPanelSwapPanes              = "panel.swap-panes"
	ActionPanelReverseSort            = "panel.reverse-sort"
	ActionPanelFilterOpen             = "panel.filter-open"
	ActionPanelToggleHidden           = "panel.toggle-hidden"
	ActionGitStage                    = "git.stage"
	ActionGitUnstage                  = "git.unstage"
	ActionBookmarkOpen                = "bookmark.open"
	ActionBookmarkAdd                 = "bookmark.add"
	ActionBookmarkDelete              = "bookmark.delete"
	ActionBookmarkOpenOther           = "bookmark.open-other"
	ActionPanelDiskUsageScan          = "panel.disk-usage-scan"
	ActionPanelDiskUsageClear         = "panel.disk-usage-clear"
	ActionPanelDirSize                = "panel.dir-size"
	ActionPanelFocusSelections        = "panel.focus-selections"
	ActionPanelOpenSelectionsRoot     = "panel.open-selections-root"
	ActionPanelSelectParentDirs       = "panel.select-parent-dirs"
	ActionPanelToggleHideInactive     = "panel.toggle-hide-inactive"
	ActionPanelExternalBrowser        = "panel.external-browser"
	ActionPanelOpenDirInOther         = "panel.open-dir-in-other"
	ActionPanelOpenActivePathInOther  = "panel.open-active-path-in-other"
	ActionPanelToggleSync             = "panel.toggle-sync"
	ActionPanelMeta                   = "panel.meta"
	ActionPanelMetaEdit               = "panel.meta-edit"
	ActionPanelComparePanels          = "panel.compare-panels"
	ActionPanelFindDuplicates         = "panel.find-duplicates"
	ActionPanelFilterDialog           = "panel.filter-dialog"

	// Pin dialog: an ad-hoc, session-only pin list of files/directories, added/removed from
	// the main panel, Find dialog, Compare view, and Dedup view.
	ActionPanelPinDialog = "panel.pin-dialog"
	ActionPanelPinToggle = "panel.pin-toggle"
	ActionPinRemove      = "pin.remove"
	ActionPinRemoveAll   = "pin.remove-all"
	ActionPinView        = "pin.view"

	// Compare view
	ActionCompareClose       = "compare.close"
	ActionCompareCycleFilter = "compare.cycle-filter"
	ActionCompareResetFilter = "compare.reset-filter"
	ActionCompareRefresh     = "compare.refresh"
	ActionCompareMerge       = "compare.merge"
	ActionCompareToggleEmpty = "compare.toggle-empty"

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
	ActionDedupPrevPane    = "dedup.prev-pane"

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
	// ActionFileMassRenameSavePattern / ActionFileMassRenameLoadPattern / ActionFileMassRenameHistory /
	// ActionFileMassRenameDeletePattern are bound via [dialog.mass_rename], not [main].
	ActionFileMassRenameSavePattern   = "file.mass-rename.save-pattern"
	ActionFileMassRenameLoadPattern   = "file.mass-rename.load-pattern"
	ActionFileMassRenameHistory       = "file.mass-rename.history"
	ActionFileMassRenameDeletePattern = "file.mass-rename.delete-pattern"
	// ActionFileRunForEachHistory is bound via [dialog.run_for_each], not [main].
	ActionFileRunForEachHistory = "file.run-for-each.history"
	ActionFileDelete            = "file.delete"
	ActionFileMkdir             = "file.mkdir"
	ActionFileMkdirOpenInOther  = "file.mkdir-open-in-other"
	// ActionFileMkdirExtractCommonName is bound via [dialog.mkdir], not [main].
	ActionFileMkdirExtractCommonName   = "file.mkdir.extract-common-name"
	ActionFileChmod                    = "file.chmod"
	ActionFileChown                    = "file.chown"
	ActionFileSymlink                  = "file.symlink"
	ActionFileHardlink                 = "file.hardlink"
	ActionFileExtract                  = "file.extract"
	ActionFileView                     = "file.view"
	ActionPreviewMenu                  = "preview.menu"
	ActionPreviewThemePicker           = "preview.theme-picker"
	ActionPreviewToggleRaw             = "preview.toggle-raw"
	ActionPreviewReload                = "preview.reload"
	ActionPreviewDiffNextHunk          = "preview.diff-next-hunk"
	ActionPreviewDiffPrevHunk          = "preview.diff-prev-hunk"
	ActionPreviewSearchStart           = "preview.search-start"
	ActionPreviewSearchNext            = "preview.search-next"
	ActionPreviewSearchPrev            = "preview.search-prev"
	ActionPreviewClose                 = "preview.close"
	ActionFileQuickView                = "file.quick-view"
	ActionFileQuickViewPreviewPageUp   = "file.quick-view.preview-page-up"
	ActionFileQuickViewPreviewPageDown = "file.quick-view.preview-page-down"
	ActionFileEdit                     = "file.edit"
	ActionFileFlatten                  = "file.flatten"

	// Copy/Move
	ActionCopy          = "file.copy"
	ActionFileDuplicate = "file.duplicate"
	ActionMove          = "file.move"

	// Remote
	ActionRemoteSFTPLink = "remote.sftp-link"

	// Jobs dialog
	ActionJobsOpen          = "jobs.open"
	ActionDedupOpen         = "dedup.open"
	ActionJobsClose         = "jobs.close"
	ActionJobsClearFinished = "jobs.clear-finished"
	ActionJobsCancel        = "jobs.cancel"
	ActionJobsPause         = "jobs.pause"
	ActionJobsResume        = "jobs.resume"
	ActionJobsQueueUp       = "jobs.queue-up"
	ActionJobsQueueDown     = "jobs.queue-down"
	ActionJobsAnswerBlocker = "jobs.answer-blocker"

	ActionJobsRateLimitIncrease = "jobs.rate-limit-increase"
	ActionJobsRateLimitDecrease = "jobs.rate-limit-decrease"
	ActionJobsRateLimitClear    = "jobs.rate-limit-clear"

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
	ActionUILeft     = "ui.left"
	ActionUIRight    = "ui.right"
	ActionUIActivate = "ui.activate"

	ActionUIOpenTheme         = "ui.open-theme"
	ActionUIOpenConfig        = "ui.open-config"
	ActionUICalibrateDebounce = "ui.calibrate-debounce"

	// ActionPreviewSettingsDialog opens the M-F3 preview settings dialog (Sixel/Kitty/Kitty-
	// placeholder confirmation checkboxes, Auto/Sixel/Kitty protocol radio, image-metadata
	// detail-level radio, and video-metadata checkbox).
	ActionPreviewSettingsDialog = "preview.settings-dialog"

	// ActionFindSelectAll marks all ranked find-dialog results and is bound via
	// [dialog.find], not [main].
	// ActionDialogInputRestoreDefault restores a focused dialog input field's suggested default
	// (Prefill) and is bound via [dialog.input], not [main].
	ActionDialogInputRestoreDefault = "ui.input.restore-default"
	// ActionDialogInputPathPickerAll opens the combined bookmarks/history/pinned path picker
	// from a focused path input and is bound via [dialog.input], not [main].
	ActionDialogInputPathPickerAll = "ui.input.path-picker-all"
	// ActionDialogInputKillWordBackward deletes back to the previous word boundary (readline C-w).
	ActionDialogInputKillWordBackward = "ui.input.kill-word-backward"
	// ActionDialogInputKillWordForward deletes up to the next word boundary (readline M-d).
	ActionDialogInputKillWordForward = "ui.input.kill-word-forward"
	// ActionDialogInputKillLine deletes the whole line into the kill buffer.
	ActionDialogInputKillLine = "ui.input.kill-line"
	// ActionDialogInputKillLineBackward deletes from the line start to the caret into the
	// kill buffer (readline C-u).
	ActionDialogInputKillLineBackward = "ui.input.kill-line-backward"
	// ActionDialogInputKillLineForward deletes from the caret to the line end into the kill
	// buffer (readline C-k).
	ActionDialogInputKillLineForward = "ui.input.kill-line-forward"
	// ActionDialogInputYank inserts the last killed text (C-w, M-d, C-u, C-k, C-l) at the caret (readline C-y).
	ActionDialogInputYank = "ui.input.yank"
	// ActionDialogInputBackwardWord moves the cursor to the previous word boundary (readline M-b).
	ActionDialogInputBackwardWord = "ui.input.backward-word"
	// ActionDialogInputForwardWord moves the cursor past the next word (readline M-f).
	ActionDialogInputForwardWord = "ui.input.forward-word"
	// ActionDialogInputLineStart moves the cursor to the start of the line (readline C-a).
	ActionDialogInputLineStart = "ui.input.line-start"
	// ActionDialogInputLineEnd moves the cursor to the end of the line (readline C-e).
	ActionDialogInputLineEnd = "ui.input.line-end"
	// ActionDialogInputUpcaseWord uppercases up to the next word end (readline M-u).
	ActionDialogInputUpcaseWord = "ui.input.upcase-word"
	// ActionDialogInputDowncaseWord lowercases up to the next word end (readline M-l).
	ActionDialogInputDowncaseWord = "ui.input.downcase-word"
	// ActionDialogInputCapitalizeWord capitalizes the next word (readline M-c; bound to M-S-u
	// because Alt+C is the dialog Cancel mnemonic).
	ActionDialogInputCapitalizeWord = "ui.input.capitalize-word"

	// ActionDestinationActivePanel / ActionDestinationInactivePanel set a dialog's destination
	// path field to the active/inactive panel path. Bound via [dialog.flatten] and
	// [dialog.transfer] (also used by the extract dialog), not [main].
	ActionDestinationActivePanel   = "ui.destination-active"
	ActionDestinationInactivePanel = "ui.destination-inactive"

	// ActionOpenInPrimary / ActionOpenInSecondary point a panel at the highlighted result in the
	// find and pin dialogs and the dedup view. Default chords: DefaultOpenInPanelKeys.
	ActionOpenInPrimary   = "ui.open-primary"
	ActionOpenInSecondary = "ui.open-secondary"
)

// Menu routing identifiers for File pulldown entries (bindable in keybindings.toml).
const (
	ActionMenuFileChattr = "menu.file.chattr"
)

// Dev menu actions (menu routing only; enabled with pc -dev).
const (
	ActionDevShowInfo      = "dev.show-info"
	ActionDevShowWarn      = "dev.show-warn"
	ActionDevShowError     = "dev.show-error"
	ActionDevDemoDeleteBar = "dev.demo-delete-bar"
)

// KnownActions lists action IDs accepted in keybindings.toml for the current app.
var KnownActions = map[string]struct{}{
	ActionAppQuit:             {},
	ActionAppQuitImmediate:    {},
	ActionAppOpenMenu:         {},
	ActionAppShowHelp:         {},
	ActionAppUserMenu:         {},
	ActionAppUserMenuEdit:     {},
	ActionAppLeaderMenu:       {},
	ActionAppCopyMenu:         {},
	ActionAppDropToShell:      {},
	ActionAppShellInsertPaths: {},

	ActionClipboardCopyFileURL:            {},
	ActionClipboardCopyDirURL:             {},
	ActionClipboardCopyFilename:           {},
	ActionClipboardCopyFilenameWithoutExt: {},

	ActionTerminalTogglePanel: {},
	ActionTerminalFocus:       {},
	ActionTerminalGrow:        {},
	ActionTerminalShrink:      {},

	ActionPanelSwitch:                 {},
	ActionPanelViMotionToggle:         {},
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
	ActionPanelGitFilterMenu:          {},
	ActionPanelHistoryBothPanels:      {},
	ActionHelpTextEditKeys:            {},
	ActionPanelFindDialog:             {},
	ActionFindView:                    {},
	ActionFindSelectAll:               {},
	ActionFindUnselectAll:             {},
	ActionFindSelectGroup:             {},
	ActionFindUnselectGroup:           {},
	ActionFindSelectParentDirs:        {},
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
	ActionPanelToggleTree:             {},
	ActionPanelTreeExpand:             {},
	ActionPanelTreeCollapse:           {},
	ActionPanelTreeCollapseAll:        {},
	ActionPanelTreeCollapseAllFull:    {},
	ActionPanelTreeExpandAllShallow:   {},
	ActionPanelTreeExpandAllFull:      {},
	ActionPanelTreePrevSiblingDir:     {},
	ActionPanelTreeNextSiblingDir:     {},
	ActionPanelToggleZoomActivePanel:  {},
	ActionPanelToggleSplitOrientation: {},
	ActionPanelSwapPanes:              {},
	ActionPanelReverseSort:            {},
	ActionPanelFilterOpen:             {},
	ActionPanelToggleHidden:           {},
	ActionGitStage:                    {},
	ActionGitUnstage:                  {},
	ActionBookmarkOpen:                {},
	ActionBookmarkAdd:                 {},
	ActionBookmarkDelete:              {},
	ActionBookmarkOpenOther:           {},
	ActionPanelDiskUsageScan:          {},
	ActionPanelDiskUsageClear:         {},
	ActionPanelDirSize:                {},
	ActionPanelFocusSelections:        {},
	ActionPanelOpenSelectionsRoot:     {},
	ActionPanelSelectParentDirs:       {},
	ActionPanelToggleHideInactive:     {},
	ActionPanelExternalBrowser:        {},
	ActionPanelOpenDirInOther:         {},
	ActionPanelOpenActivePathInOther:  {},
	ActionPanelToggleSync:             {},
	ActionPanelMeta:                   {},
	ActionPanelMetaEdit:               {},
	ActionPanelComparePanels:          {},
	ActionPanelFindDuplicates:         {},
	ActionPanelFilterDialog:           {},

	ActionPanelPinDialog:  {},
	ActionPanelPinToggle:  {},
	ActionOpenInPrimary:   {},
	ActionOpenInSecondary: {},
	ActionPinRemove:       {},
	ActionPinRemoveAll:    {},
	ActionPinView:         {},

	ActionCompareClose:       {},
	ActionCompareCycleFilter: {},
	ActionCompareResetFilter: {},
	ActionCompareRefresh:     {},
	ActionCompareMerge:       {},
	ActionCompareToggleEmpty: {},

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
	ActionDedupPrevPane:    {},

	ActionDialogConfirm: {},
	ActionDialogCancel:  {},
	ActionDialogNext:    {},
	ActionDialogPrev:    {},

	ActionFileRename:                   {},
	ActionFileRenameOpenSanitize:       {},
	ActionFileRenameOpenSlugify:        {},
	ActionFileRenameOpenEncoding:       {},
	ActionFileMassRenameSavePattern:    {},
	ActionFileMassRenameLoadPattern:    {},
	ActionFileMassRenameHistory:        {},
	ActionFileMassRenameDeletePattern:  {},
	ActionFileRunForEachHistory:        {},
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
	ActionPreviewMenu:                  {},
	ActionPreviewThemePicker:           {},
	ActionPreviewToggleRaw:             {},
	ActionPreviewReload:                {},
	ActionPreviewDiffNextHunk:          {},
	ActionPreviewDiffPrevHunk:          {},
	ActionPreviewSearchStart:           {},
	ActionPreviewSearchNext:            {},
	ActionPreviewSearchPrev:            {},
	ActionPreviewClose:                 {},
	ActionFileQuickView:                {},
	ActionFileQuickViewPreviewPageUp:   {},
	ActionFileQuickViewPreviewPageDown: {},
	ActionFileEdit:                     {},
	ActionFileFlatten:                  {},

	ActionCopy:          {},
	ActionFileDuplicate: {},
	ActionMove:          {},

	ActionRemoteSFTPLink: {},

	ActionMenuFileChattr: {},

	ActionJobsOpen:          {},
	ActionDedupOpen:         {},
	ActionJobsClose:         {},
	ActionJobsClearFinished: {},
	ActionJobsCancel:        {},
	ActionJobsPause:         {},
	ActionJobsResume:        {},
	ActionJobsQueueUp:       {},
	ActionJobsQueueDown:     {},
	ActionJobsAnswerBlocker: {},

	ActionJobsRateLimitIncrease: {},
	ActionJobsRateLimitDecrease: {},
	ActionJobsRateLimitClear:    {},

	ActionCommandsOpen:      {},
	ActionCommandsClose:     {},
	ActionCommandsTerminate: {},
	ActionCommandsKill:      {},
	ActionFileRunForEach:    {},

	ActionMessagesOpen:  {},
	ActionMessagesClose: {},
	ActionMessagesClear: {},

	ActionUIOpenTheme:           {},
	ActionUIOpenConfig:          {},
	ActionUICalibrateDebounce:   {},
	ActionPreviewSettingsDialog: {},

	ActionDialogInputRestoreDefault: {},
	ActionDialogInputPathPickerAll:  {},

	ActionDialogInputKillWordBackward: {},
	ActionDialogInputKillWordForward:  {},
	ActionDialogInputKillLine:         {},
	ActionDialogInputKillLineBackward: {},
	ActionDialogInputKillLineForward:  {},
	ActionDialogInputYank:             {},
	ActionDialogInputBackwardWord:     {},
	ActionDialogInputForwardWord:      {},
	ActionDialogInputLineStart:        {},
	ActionDialogInputLineEnd:          {},
	ActionDialogInputUpcaseWord:       {},
	ActionDialogInputDowncaseWord:     {},
	ActionDialogInputCapitalizeWord:   {},

	ActionDestinationActivePanel:   {},
	ActionDestinationInactivePanel: {},
}
