package keymap

// ActionSpec describes one configurable action: its stable ID, human-friendly
// display metadata, default chord strings, and search keywords.
type ActionSpec struct {
	// ID is the stable TOML action identifier (e.g. "file.copy").
	ID string
	// Title is a short human-readable name (e.g. "Copy").
	Title string
	// Section groups actions in the help screen (e.g. "File operations").
	Section string
	// DefaultKeys is the built-in chord strings (e.g. []string{"F5"}); empty = unbound by default.
	DefaultKeys []string
	// PreferredKey is the visual label shown in menus, footer, and help summary.
	// When empty, the first DefaultKey (if any) is used.
	PreferredKey string
	// Keywords are extra fuzzy-search terms for the help screen.
	Keywords []string
}

// DefaultActionSpecs returns all known configurable actions in display order.
func DefaultActionSpecs() []ActionSpec {
	return []ActionSpec{
		// ── App ──
		{
			ID:           ActionAppQuit,
			Title:        "Quit",
			Section:      "App",
			DefaultKeys:  []string{"F10"},
			PreferredKey: "F10",
			Keywords:     []string{"exit", "close"},
		},
		{
			ID:           ActionAppQuitImmediate,
			Title:        "Quit without confirmation",
			Section:      "App",
			DefaultKeys:  []string{"S-F10"},
			PreferredKey: "S-F10",
			Keywords:     []string{"exit", "close", "force", "kill"},
		},
		{
			ID:           ActionAppOpenMenu,
			Title:        "Open menu",
			Section:      "App",
			DefaultKeys:  []string{"F9"},
			PreferredKey: "F9",
			Keywords:     []string{"menu bar"},
		},
		{
			ID:           ActionAppShowHelp,
			Title:        "Help",
			Section:      "App",
			DefaultKeys:  []string{"F1"},
			PreferredKey: "F1",
			Keywords:     []string{"help screen"},
		},
		{
			ID:           ActionAppUserMenu,
			Title:        "User menu",
			Section:      "App",
			DefaultKeys:  []string{"F2"},
			PreferredKey: "F2",
			Keywords:     []string{"menu.toml", "custom commands"},
		},
		{
			ID:           ActionAppUserMenuEdit,
			Title:        "Edit user menu",
			Section:      "App",
			DefaultKeys:  []string{"S-F2"},
			PreferredKey: "S-F2",
			Keywords:     []string{"menu.toml", "editor", "custom commands"},
		},

		// ── Panel navigation ──
		{
			ID:          ActionPanelSwitch,
			Title:       "Switch panel",
			Section:     "Navigation",
			DefaultKeys: []string{"tab"},
			Keywords:    []string{"focus", "toggle panel"},
		},
		{
			ID:           ActionPanelFocusSelections,
			Title:        "Focus selections panel",
			Section:      "Navigation",
			DefaultKeys:  []string{"C-M-s"},
			PreferredKey: "C-M-s",
			Keywords:     []string{"selections", "strip"},
		},
		{
			ID:          ActionPanelToggleHideInactive,
			Title:       "Hide inactive panel",
			Section:     "Navigation",
			DefaultKeys: []string{"S-tab"},
			Keywords:    []string{"shift-tab", "single panel", "hide panel"},
		},
		{
			ID:          ActionNavUp,
			Title:       "Cursor up",
			Section:     "Navigation",
			DefaultKeys: []string{"up"},
			Keywords:    []string{"previous"},
		},
		{
			ID:          ActionNavDown,
			Title:       "Cursor down",
			Section:     "Navigation",
			DefaultKeys: []string{"down"},
			Keywords:    []string{"next"},
		},
		{
			ID:          ActionNavPageUp,
			Title:       "Page up",
			Section:     "Navigation",
			DefaultKeys: []string{"pgup"},
			Keywords:    []string{"scroll"},
		},
		{
			ID:          ActionNavPageDown,
			Title:       "Page down",
			Section:     "Navigation",
			DefaultKeys: []string{"pgdn"},
			Keywords:    []string{"scroll"},
		},
		{
			ID:          ActionNavTop,
			Title:       "First entry",
			Section:     "Navigation",
			DefaultKeys: []string{"home"},
			Keywords:    []string{"top", "beginning"},
		},
		{
			ID:          ActionNavBottom,
			Title:       "Last entry",
			Section:     "Navigation",
			DefaultKeys: []string{"end"},
			Keywords:    []string{"bottom"},
		},
		{
			ID:           ActionNavOpen,
			Title:        "Open directory or file",
			Section:      "Navigation",
			DefaultKeys:  []string{"enter", "right"},
			PreferredKey: "Enter",
			Keywords:     []string{"open", "select", "xdg-open", "open file"},
		},
		{
			ID:           ActionNavParent,
			Title:        "Parent directory",
			Section:      "Navigation",
			DefaultKeys:  []string{"left", "backspace"},
			PreferredKey: "←",
			Keywords:     []string{"up", "back"},
		},
		{
			ID:           ActionNavHome,
			Title:        "Home directory",
			Section:      "Navigation",
			DefaultKeys:  []string{"~", "§"},
			PreferredKey: "~",
			Keywords:     []string{"home", "~", "cd"},
		},
		{
			ID:          ActionNavForward,
			Title:       "Forward history",
			Section:     "Navigation",
			DefaultKeys: []string{"M-C-left"},
			Keywords:    []string{"back", "re-enter"},
		},
		{
			ID:          ActionNavBackward,
			Title:       "Backward history",
			Section:     "Navigation",
			DefaultKeys: []string{"M-C-right"},
			Keywords:    []string{"forward", "re-enter", "timeline"},
		},
		{
			ID:           ActionPanelHistoryDialog,
			Title:        "Directory history",
			Section:      "Navigation",
			DefaultKeys:  []string{"M-h", "C-h"},
			PreferredKey: "M-h",
			Keywords:     []string{"history", "picker", "navigate", "alt-h"},
		},
		{
			ID:           ActionPanelFindDialog,
			Title:        "Find files",
			Section:      "Navigation",
			DefaultKeys:  []string{"C-f"},
			PreferredKey: "C-f",
			Keywords:     []string{"find", "search", "recursive", "fuzzy", "locate"},
		},
		{
			ID:           ActionPanelRefresh,
			Title:        "Refresh panel",
			Section:      "Navigation",
			DefaultKeys:  []string{"M-C-r"},
			PreferredKey: "M-C-r",
			Keywords:     []string{"reload"},
		},
		{
			ID:           ActionPanelExternalBrowser,
			Title:        "External browser",
			Section:      "Navigation",
			DefaultKeys:  []string{"M-e"},
			PreferredKey: "M-e",
			Keywords:     []string{"xdg-open", "gui", "file manager", "finder"},
		},
		{
			ID:           ActionPanelOpenDirInOther,
			Title:        "Open directory in other panel",
			Section:      "Navigation",
			DefaultKeys:  []string{"M-o"},
			PreferredKey: "M-o",
			Keywords:     []string{"inactive", "split", "cd", "other panel"},
		},
		{
			ID:           ActionPanelOpenActivePathInOther,
			Title:        "Open current path in other panel",
			Section:      "Navigation",
			DefaultKeys:  []string{"M-i"},
			PreferredKey: "M-i",
			Keywords:     []string{"inactive", "split", "cd", "cwd", "other panel"},
		},
		{
			ID:           ActionPanelToggleSync,
			Title:        "Toggle panel sync",
			Section:      "Navigation",
			DefaultKeys:  []string{"C-M-o"},
			PreferredKey: "C-M-o",
			Keywords:     []string{"sync", "follow", "mirror", "latch", "other panel"},
		},

		// ── Disk usage ──
		{
			ID:          ActionPanelDiskUsageScan,
			Title:       "Disk usage scan",
			Section:     "Disk usage",
			DefaultKeys: []string{"C-d"},
			Keywords:    []string{"size", "subtree"},
		},
		{
			ID:          ActionPanelDiskUsageAbortAll,
			Title:       "Abort disk usage scans",
			Section:     "Disk usage",
			DefaultKeys: []string{"C-M-d"},
			Keywords:    []string{"cancel", "stop"},
		},
		{
			ID:          ActionPanelDiskUsageClear,
			Title:       "Clear disk usage data",
			Section:     "Disk usage",
			DefaultKeys: []string{"M-d"},
			Keywords:    []string{"reset", "forget", "cache"},
		},

		// ── Selection ──
		{
			ID:          ActionPanelSelectToggle,
			Title:       "Toggle selection",
			Section:     "Selection",
			DefaultKeys: []string{"insert"},
			Keywords:    []string{"select", "mark"},
		},
		{
			ID:          ActionPanelSelectGroup,
			Title:       "Select group",
			Section:     "Selection",
			DefaultKeys: []string{"+"},
			Keywords:    []string{"pattern", "glob"},
		},
		{
			ID:          ActionPanelUnselectGroup,
			Title:       "Unselect group",
			Section:     "Selection",
			DefaultKeys: []string{"-"},
			Keywords:    []string{"pattern", "glob", "deselect"},
		},
		{
			ID:          ActionPanelInvertSelection,
			Title:       "Invert selection",
			Section:     "Selection",
			DefaultKeys: []string{"*"},
			Keywords:    []string{"reverse"},
		},
		{
			ID:          ActionPanelClearSelection,
			Title:       "Clear selection",
			Section:     "Selection",
			DefaultKeys: []string{"C-u"},
			Keywords:    []string{"deselect", "unmark"},
		},
		{
			ID:           ActionPanelStashToggle,
			Title:        "Toggle selection stash",
			Section:      "Selection",
			DefaultKeys:  []string{"M-insert"},
			PreferredKey: "M-insert",
			Keywords:     []string{"stash", "clipboard", "buffer", "selection"},
		},

		// ── Sort & display ──
		{
			ID:          ActionPanelSortDialog,
			Title:       "Sort dialog",
			Section:     "Sort & display",
			DefaultKeys: []string{"C-s"},
			Keywords:    []string{"order"},
		},
		{
			ID:          ActionPanelListingFormatDialog,
			Title:       "Listing format dialog",
			Section:     "Sort & display",
			DefaultKeys: nil, // opened from Left/Right menu by default
			Keywords:    []string{"columns", "listing", "mtime", "permissions", "brief", "radio"},
		},
		{
			ID:          ActionPanelCycleSort,
			Title:       "Cycle sort mode",
			Section:     "Sort & display",
			DefaultKeys: nil, // unbound by default
			Keywords:    []string{"order"},
		},
		{
			ID:          ActionPanelCycleListingFormat,
			Title:       "Cycle listing format",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-t"},
			Keywords:    []string{"columns", "listing", "mtime", "permissions", "brief"},
		},
		{
			ID:          ActionPanelToggleCarousel,
			Title:       "Toggle carousel view",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-c"},
			Keywords:    []string{"carousel", "columns", "preview", "parent", "child", "godu"},
		},
		{
			ID:          ActionPanelToggleZoomActivePanel,
			Title:       "Toggle zoom active panel (runtime)",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-z"},
			Keywords:    []string{"zoom", "layout", "wide", "column", "split"},
		},
		{
			ID:          ActionPanelReverseSort,
			Title:       "Reverse sort",
			Section:     "Sort & display",
			DefaultKeys: nil, // unbound by default
			Keywords:    []string{"order", "direction"},
		},
		{
			ID:          ActionPanelToggleHidden,
			Title:       "Toggle hidden files",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-."},
			Keywords:    []string{"show", "hide", "dotfiles", "gitignore", "ignored"},
		},
		{
			ID:          ActionPanelMeta,
			Title:       "Meta column",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-,"},
			Keywords:    []string{"meta", "column", "command", "count"},
		},
		{
			ID:           ActionPanelMetaEdit,
			Title:        "Edit meta commands",
			Section:      "Sort & display",
			DefaultKeys:  []string{"S-M-,", "M-;"},
			PreferredKey: "S-M-,",
			Keywords:     []string{"meta", "meta.toml", "editor", "column", "command"},
		},

		// ── Bookmarks ──
		{
			ID:           ActionBookmarkOpen,
			Title:        "Open bookmarks",
			Section:      "Bookmarks",
			DefaultKeys:  []string{"C-g", "C-e"},
			PreferredKey: "C-g",
			Keywords:     []string{"fzf-marks", "marks", "picker", "path-picker", "history", "destination"},
		},
		{
			ID:          ActionBookmarkAdd,
			Title:       "Add bookmark",
			Section:     "Bookmarks",
			DefaultKeys: []string{"M-m"},
			Keywords:    []string{"mark", "save"},
		},
		{
			ID:          ActionBookmarkDelete,
			Title:       "Delete bookmark",
			Section:     "Bookmarks",
			DefaultKeys: nil, // overlay: DefaultBookmarkDialogOverlayKeys
			Keywords:    []string{"fzf-marks", "mark", "remove", "delete"},
		},
		{
			ID:           ActionFindSelectAll,
			Title:        "Select all",
			Section:      "Find",
			DefaultKeys:  nil, // overlay: DefaultFindDialogOverlayKeys
			PreferredKey: "F5",
			Keywords:     []string{"find dialog", "mark all"},
		},
		{
			ID:          ActionRemoteSFTPLink,
			Title:       "SFTP ...",
			Section:     "Remote",
			DefaultKeys: []string{"C-r"},
			Keywords:    []string{"ssh", "sftp", "remote", "connect"},
		},

		// ── File operations ──
		{
			ID:           ActionFileRename,
			Title:        "Rename / Move",
			Section:      "File operations",
			DefaultKeys:  []string{"S-F6"},
			PreferredKey: "S-F6",
			Keywords:     []string{"rename", "move"},
		},
		{
			ID:          ActionFileMkdir,
			Title:       "Create directory",
			Section:     "File operations",
			DefaultKeys: []string{"F7"},
			Keywords:    []string{"mkdir", "folder"},
		},
		{
			ID:           ActionFileMkdirOpenInOther,
			Title:        "Create directory and open in other",
			Section:      "File operations",
			DefaultKeys:  []string{"S-F7"},
			PreferredKey: "S-F7",
			Keywords:     []string{"mkdir", "folder", "other", "inactive"},
		},
		{
			ID:          ActionFileDelete,
			Title:       "Delete",
			Section:     "File operations",
			DefaultKeys: []string{"F8", "delete"},
			Keywords:    []string{"remove", "trash"},
		},
		{
			ID:          ActionFileChmod,
			Title:       "Change mode",
			Section:     "File operations",
			DefaultKeys: nil, // unbound by default (menu only)
			Keywords:    []string{"permissions", "chmod"},
		},
		{
			ID:          ActionFileChown,
			Title:       "Change owner",
			Section:     "File operations",
			DefaultKeys: nil, // unbound by default (menu only)
			Keywords:    []string{"owner", "group", "chown"},
		},
		{
			ID:          ActionFileSymlink,
			Title:       "Symlink",
			Section:     "File operations",
			DefaultKeys: nil, // unbound by default (menu only)
			Keywords:    []string{"link", "symbolic"},
		},
		{
			ID:          ActionFileHardlink,
			Title:       "Hard link",
			Section:     "File operations",
			DefaultKeys: nil, // unbound by default (menu only)
			Keywords:    []string{"link"},
		},
		{
			ID:          ActionFileView,
			Title:       "Full screen file view",
			Section:     "File operations",
			DefaultKeys: []string{"F3"},
			Keywords:    []string{"viewer", "view", "fullscreen", "bat"},
		},
		{
			ID:          ActionFileQuickView,
			Title:       "Quick view",
			Section:     "File operations",
			DefaultKeys: []string{"S-F3"},
			Keywords:    []string{"preview", "inactive", "bat"},
		},
		{
			ID:          ActionMenuFileViewPath,
			Title:       "View file path",
			Section:     "File operations",
			DefaultKeys: nil,
			Keywords:    []string{"menu"},
		},
		{
			ID:          ActionFileEdit,
			Title:       "Edit file",
			Section:     "File operations",
			DefaultKeys: []string{"F4"},
			Keywords:    []string{"editor", "edit"},
		},
		{
			ID:          ActionMenuFileRelativeSymlink,
			Title:       "Relative symlink",
			Section:     "File operations",
			DefaultKeys: nil,
			Keywords:    []string{"menu"},
		},
		{
			ID:          ActionMenuFileEditSymlink,
			Title:       "Edit symlink",
			Section:     "File operations",
			DefaultKeys: nil,
			Keywords:    []string{"menu"},
		},
		{
			ID:          ActionMenuFileChattr,
			Title:       "Change file attributes",
			Section:     "File operations",
			DefaultKeys: nil,
			Keywords:    []string{"chattr", "menu"},
		},
		{
			ID:          ActionCopy,
			Title:       "Copy",
			Section:     "File operations",
			DefaultKeys: []string{"F5"},
			Keywords:    []string{"duplicate"},
		},
		{
			ID:           ActionFileCopyHere,
			Title:        "Copy here",
			Section:      "File operations",
			DefaultKeys:  []string{"S-F5"},
			PreferredKey: "S-F5",
			Keywords:     []string{"duplicate", "directory", "same directory"},
		},
		{
			ID:          ActionFileExtract,
			Title:       "Extract archives",
			Section:     "File operations",
			DefaultKeys: nil,
			Keywords:    []string{"unpack", "unzip", "tar", "archive", "decompress"},
		},
		{
			ID:          ActionMove,
			Title:       "Move",
			Section:     "File operations",
			DefaultKeys: []string{"F6"},
			Keywords:    []string{"move"},
		},
		{
			ID:           ActionFileFlatten,
			Title:        "Flatten directories",
			Section:      "File operations",
			DefaultKeys:  []string{"C-M-f"},
			PreferredKey: "C-M-f",
			Keywords:     []string{"flatten", "hoist", "directory"},
		},

		// ── Commands ──
		{
			ID:           ActionCommandsOpen,
			Title:        "Open Commands view",
			Section:      "Commands",
			DefaultKeys:  []string{"C-k"},
			PreferredKey: "C-k",
			Keywords:     []string{"shell", "stdout", "stderr"},
		},
		{
			ID:          ActionCommandsClose,
			Title:       "Close Commands view",
			Section:     "Commands",
			DefaultKeys: nil, // overlay: DefaultCommandsOverlayKeys
			Keywords:    []string{"back", "browser", "esc"},
		},
		{
			ID:          ActionFileRunForEach,
			Title:       "Run for each",
			Section:     "Commands",
			DefaultKeys: nil,
			Keywords:    []string{"exec", "subprocess", "batch"},
		},

		// ── Messages (status / toast log) ──
		{
			ID:           ActionMessagesOpen,
			Title:        "Open Messages view",
			Section:      "Messages",
			DefaultKeys:  []string{"C-M-l"},
			PreferredKey: "C-M-l",
			Keywords:     []string{"log", "toast", "status", "banner"},
		},
		{
			ID:          ActionMessagesClose,
			Title:       "Close Messages view",
			Section:     "Messages",
			DefaultKeys: nil, // overlay: DefaultMessagesOverlayKeys
			Keywords:    []string{"back", "browser", "esc"},
		},
		{
			ID:          ActionMessagesClear,
			Title:       "Clear messages",
			Section:     "Messages",
			DefaultKeys: nil,
			Keywords:    []string{"log", "toast", "status"},
		},

		// ── Jobs ──
		// jobs.open defaults belong in [action_keys] (global). Other
		// jobs.* defaults live in DefaultJobsOverlayKeys ([jobs_action_keys]).
		{
			ID:           ActionJobsOpen,
			Title:        "Open jobs view",
			Section:      "Jobs",
			DefaultKeys:  []string{"C-j"},
			PreferredKey: "C-j",
			Keywords:     []string{"queue", "background"},
		},
		{
			ID:           ActionJobsAnswerBlocker,
			Title:        "Answer job blocker",
			Section:      "Jobs",
			DefaultKeys:  []string{"C-q", "M-q"},
			PreferredKey: "C-q",
			Keywords:     []string{"blocker", "waiting", "conflict"},
		},
		{
			ID:          ActionJobsCancel,
			Title:       "Cancel job",
			Section:     "Jobs",
			DefaultKeys: nil,
			Keywords:    []string{"abort"},
		},
		{
			ID:          ActionJobsPause,
			Title:       "Pause queued job",
			Section:     "Jobs",
			DefaultKeys: nil,
			Keywords:    []string{"hold"},
		},
		{
			ID:          ActionJobsResume,
			Title:       "Resume paused job",
			Section:     "Jobs",
			DefaultKeys: nil,
			Keywords:    []string{"unpause", "start"},
		},
		{
			ID:          ActionJobsQueueUp,
			Title:       "Move job up in queue",
			Section:     "Jobs",
			DefaultKeys: nil,
			Keywords:    []string{"reorder"},
		},
		{
			ID:          ActionJobsQueueDown,
			Title:       "Move job down in queue",
			Section:     "Jobs",
			DefaultKeys: nil,
			Keywords:    []string{"reorder"},
		},
		{
			ID:          ActionJobsClose,
			Title:       "Close jobs view",
			Section:     "Jobs",
			DefaultKeys: nil, // overlay: DefaultJobsOverlayKeys
			Keywords:    []string{"back", "browser", "esc"},
		},
		{
			ID:          ActionJobsClearFinished,
			Title:       "Clear finished jobs",
			Section:     "Jobs",
			DefaultKeys: nil,
			Keywords:    []string{"remove", "done"},
		},

		// ── Options dialogs ──
		{
			ID:          ActionUIOpenTheme,
			Title:       "Theme picker",
			Section:     "UI",
			DefaultKeys: nil,
			Keywords:    []string{"appearance", "colors"},
		},
		{
			ID:          ActionUIOpenConfig,
			Title:       "Configuration editor",
			Section:     "UI",
			DefaultKeys: nil,
			Keywords:    []string{"settings", "toml"},
		},
		{
			ID:           ActionDialogInputRestoreDefault,
			Title:        "Restore default placeholder",
			Section:      "UI",
			DefaultKeys:  nil, // bound only via [dialog_input_action_keys] (defaults C-r and C-d)
			PreferredKey: "C-r",
			Keywords:     []string{"restore", "default", "placeholder", "prefill", "suggested"},
		},
		{
			ID:           ActionDialogInputKillWordBackward,
			Title:        "Delete previous word in dialog input",
			Section:      "UI",
			DefaultKeys:  nil,
			PreferredKey: "C-w",
			Keywords:     []string{"backward", "kill", "word", "path", "input"},
		},
		{
			ID:           ActionDialogInputBackwardWord,
			Title:        "Move backward by word in dialog input",
			Section:      "UI",
			DefaultKeys:  nil,
			PreferredKey: "M-b",
			Keywords:     []string{"backward", "word", "path", "input"},
		},
		{
			ID:           ActionDialogInputForwardWord,
			Title:        "Move forward by word in dialog input",
			Section:      "UI",
			DefaultKeys:  nil,
			PreferredKey: "M-f",
			Keywords:     []string{"forward", "word", "path", "input"},
		},

		// ── Filter (unbound by default) ──
		{
			ID:          ActionPanelFilterOpen,
			Title:       "Open quick filter",
			Section:     "Filter",
			DefaultKeys: nil, // unbound by default
			Keywords:    []string{"search", "find", "fuzzy"},
		},
	}
}

var specByActionID map[string]ActionSpec

func init() {
	specs := DefaultActionSpecs()
	specByActionID = make(map[string]ActionSpec, len(specs))
	for _, s := range specs {
		specByActionID[s.ID] = s
	}
}

// SpecForAction returns the built-in ActionSpec for id, if any.
func SpecForAction(id string) (ActionSpec, bool) {
	s, ok := specByActionID[id]
	return s, ok
}
