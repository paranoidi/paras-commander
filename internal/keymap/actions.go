package keymap

// Action identifiers match docs/keybindings.md (single-stroke bindings only in v1).
const (
	ActionAppQuit     = "app.quit"
	ActionAppOpenMenu = "app.open-menu"
	ActionAppShowHelp = "app.show-help"

	ActionPanelSwitch        = "panel.switch"
	ActionNavUp              = "nav.up"
	ActionNavDown            = "nav.down"
	ActionNavPageUp          = "nav.page-up"
	ActionNavPageDown        = "nav.page-down"
	ActionNavTop             = "nav.top"
	ActionNavBottom          = "nav.bottom"
	ActionNavOpen            = "nav.open"
	ActionNavParent          = "nav.parent"
	ActionNavHome            = "nav.home"
	ActionNavForward         = "nav.forward"
	ActionNavBackward        = "nav.backward"
	ActionPanelHistoryDialog = "panel.history-dialog"
	ActionPanelRefresh           = "panel.refresh"
	ActionPanelSelectToggle      = "panel.select-toggle"
	ActionPanelSelectGroup       = "panel.select-group"
	ActionPanelUnselectGroup     = "panel.unselect-group"
	ActionPanelInvertSelection   = "panel.invert-selection"
	ActionPanelClearSelection    = "panel.clear-selection"
	ActionPanelSortDialog        = "panel.sort-dialog"
	ActionPanelCycleSort         = "panel.cycle-sort"
	ActionPanelReverseSort       = "panel.reverse-sort"
	ActionPanelFilterOpen        = "panel.filter-open"
	ActionPanelToggleHidden      = "panel.toggle-hidden"
	ActionBookmarkOpen           = "bookmark.open"
	ActionBookmarkAdd            = "bookmark.add"
	ActionPanelDiskUsageScan     = "panel.disk-usage-scan"
	ActionPanelDiskUsageAbortAll = "panel.disk-usage-abort-all"
	ActionPanelFocusSelections   = "panel.focus-selections"
	ActionPanelExternalBrowser   = "panel.external-browser"
	ActionPanelOpenDirInOther    = "panel.open-dir-in-other"
	ActionPanelToggleSync        = "panel.toggle-sync"
	ActionPanelMeta              = "panel.meta"

	// Dialog actions
	ActionDialogConfirm = "ui.confirm"
	ActionDialogCancel  = "ui.cancel"
	ActionDialogNext    = "ui.next-field"
	ActionDialogPrev    = "ui.prev-field"

	// File operations
	ActionFileRename   = "file.rename"
	ActionFileDelete   = "file.delete"
	ActionFileMkdir    = "file.mkdir"
	ActionFileChmod    = "file.chmod"
	ActionFileChown    = "file.chown"
	ActionFileSymlink  = "file.symlink"
	ActionFileHardlink = "file.hardlink"
	ActionFileView     = "file.view"
	ActionFileEdit     = "file.edit"

	// Copy/Move
	ActionCopy = "file.copy"
	ActionMove = "file.move"

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

	// Commands screen + external command execution
	ActionCommandsOpen = "commands.open"
	ActionCommandsClose = "commands.close"
	ActionFileRunForEach = "file.run-for-each"

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
)

// Menu routing identifiers for File pulldown entries (bindable in keybindings.toml).
const (
	ActionMenuFileViewPath        = "menu.file.view-path"
	ActionMenuFileFilteredView    = "menu.file.filtered-view"
	ActionMenuFileRelativeSymlink = "menu.file.relative-symlink"
	ActionMenuFileEditSymlink     = "menu.file.edit-symlink"
	ActionMenuFileAdvancedChown   = "menu.file.advanced-chown"
	ActionMenuFileChattr          = "menu.file.chattr"
)

// KnownActions lists action IDs accepted in keybindings.toml for the current app.
var KnownActions = map[string]struct{}{
	ActionAppQuit:     {},
	ActionAppOpenMenu: {},
	ActionAppShowHelp: {},

	ActionPanelSwitch:        {},
	ActionNavUp:              {},
	ActionNavDown:            {},
	ActionNavPageUp:          {},
	ActionNavPageDown:        {},
	ActionNavTop:             {},
	ActionNavBottom:          {},
	ActionNavOpen:            {},
	ActionNavParent:          {},
	ActionNavHome:            {},
	ActionNavForward:         {},
	ActionNavBackward:        {},
	ActionPanelHistoryDialog: {},
	ActionPanelRefresh:           {},
	ActionPanelSelectToggle:      {},
	ActionPanelSelectGroup:       {},
	ActionPanelUnselectGroup:     {},
	ActionPanelInvertSelection:   {},
	ActionPanelClearSelection:    {},
	ActionPanelSortDialog:        {},
	ActionPanelCycleSort:         {},
	ActionPanelReverseSort:       {},
	ActionPanelFilterOpen:        {},
	ActionPanelToggleHidden:      {},
	ActionBookmarkOpen:           {},
	ActionBookmarkAdd:            {},
	ActionPanelDiskUsageScan:     {},
	ActionPanelDiskUsageAbortAll: {},
	ActionPanelFocusSelections:   {},
	ActionPanelExternalBrowser:   {},
	ActionPanelOpenDirInOther:    {},
	ActionPanelToggleSync:        {},
	ActionPanelMeta:              {},

	ActionDialogConfirm: {},
	ActionDialogCancel:  {},
	ActionDialogNext:    {},
	ActionDialogPrev:    {},

	ActionFileRename:   {},
	ActionFileDelete:   {},
	ActionFileMkdir:    {},
	ActionFileChmod:    {},
	ActionFileChown:    {},
	ActionFileSymlink:  {},
	ActionFileHardlink: {},
	ActionFileView:     {},
	ActionFileEdit:     {},

	ActionCopy: {},
	ActionMove: {},

	ActionMenuFileViewPath:        {},
	ActionMenuFileFilteredView:    {},
	ActionMenuFileRelativeSymlink: {},
	ActionMenuFileEditSymlink:     {},
	ActionMenuFileAdvancedChown:   {},
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

	ActionCommandsOpen:  {},
	ActionCommandsClose: {},
	ActionFileRunForEach: {},

	ActionUIOpenTheme:  {},
	ActionUIOpenConfig: {},
}
