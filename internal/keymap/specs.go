package keymap

// HelpViews is a bitmask of views whose F1 help lists the action.
// The zero value means the action appears in no F1 help — used for
// dialog-overlay-only shortcuts documented by the dialog itself.
type HelpViews uint16

const (
	HelpBrowser HelpViews = 1 << iota
	HelpJobs
	HelpCommands
	HelpMessages
	HelpCompare
	HelpDedup
	HelpFilePreview
)

// Tagging shorthands for DefaultActionSpecs.
const (
	helpAllViews      = HelpBrowser | HelpJobs | HelpCommands | HelpMessages | HelpCompare | HelpDedup | HelpFilePreview
	helpAllButPreview = helpAllViews &^ HelpFilePreview // menu bar and list nav are absent in file preview
)

// ActionSpec describes one configurable action: its stable ID, human-friendly
// display metadata, default chord strings, and search keywords.
type ActionSpec struct {
	// ID is the stable TOML action identifier (e.g. "file.copy").
	ID string
	// Title is a short human-readable name (e.g. "Copy").
	Title string
	// Section groups actions in the help screen (e.g. "File operations").
	Section string
	// Views selects which views' F1 help lists the action (zero = none).
	Views HelpViews
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
			Views:        helpAllViews,
			Title:        "Quit",
			Section:      "App",
			DefaultKeys:  []string{"F10"},
			PreferredKey: "F10",
			Keywords:     []string{"exit", "close"},
		},
		{
			ID:           ActionAppQuitImmediate,
			Views:        helpAllViews,
			Title:        "Quit without confirmation",
			Section:      "App",
			DefaultKeys:  []string{"S-F10"},
			PreferredKey: "S-F10",
			Keywords:     []string{"exit", "close", "force", "kill"},
		},
		{
			ID:           ActionAppOpenMenu,
			Views:        helpAllButPreview,
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
			Views:        HelpBrowser | HelpFilePreview,
			Title:        "User menu",
			Section:      "App",
			DefaultKeys:  []string{"F2"},
			PreferredKey: "F2",
			Keywords:     []string{"menu.toml", "custom commands"},
		},
		{
			ID:           ActionAppUserMenuEdit,
			Views:        HelpBrowser | HelpFilePreview,
			Title:        "Edit user menu",
			Section:      "App",
			DefaultKeys:  []string{"S-F2"},
			PreferredKey: "S-F2",
			Keywords:     []string{"menu.toml", "editor", "custom commands"},
		},
		{
			ID:           ActionAppDropToShell,
			Views:        HelpBrowser,
			Title:        "Drop to shell",
			Section:      "Shell",
			DefaultKeys:  []string{"C-o"},
			PreferredKey: "C-o",
			Keywords:     []string{"shell", "subshell", "ctrl-o", "terminal"},
		},
		{
			ID:           ActionAppShellInsertPaths,
			Views:        HelpBrowser,
			Title:        "Send paths to shell",
			Section:      "Shell",
			DefaultKeys:  []string{"M-Enter", "S-Enter"},
			PreferredKey: "M-Enter",
			Keywords:     []string{"shell", "insert", "filename", "command line", "alt-enter", "shift-enter", "inject"},
		},

		// ── Shell panel ──
		{
			ID:           ActionTerminalTogglePanel,
			Views:        HelpBrowser,
			Title:        "Show/hide shell panel",
			Section:      "Shell",
			DefaultKeys:  []string{"C-M-p"},
			PreferredKey: "C-M-p",
			Keywords:     []string{"terminal", "shell", "panel", "strip", "embedded"},
		},
		{
			ID:           ActionTerminalFocus,
			Views:        HelpBrowser,
			Title:        "Focus shell",
			Section:      "Shell",
			DefaultKeys:  []string{"M-p"},
			PreferredKey: "M-p",
			Keywords:     []string{"terminal", "shell", "focus", "switch", "panel"},
		},
		{
			ID:           ActionTerminalGrow,
			Views:        HelpBrowser,
			Title:        "Grow shell panel",
			Section:      "Shell",
			DefaultKeys:  nil, // overlay: DefaultTerminalOverlayKeys
			PreferredKey: "C-k",
			Keywords:     []string{"terminal", "resize", "taller", "grow"},
		},
		{
			ID:           ActionTerminalShrink,
			Views:        HelpBrowser,
			Title:        "Shrink shell panel",
			Section:      "Shell",
			DefaultKeys:  nil, // overlay: DefaultTerminalOverlayKeys
			PreferredKey: "C-j",
			Keywords:     []string{"terminal", "resize", "shorter", "shrink"},
		},

		// ── Panel navigation ──
		{
			ID:          ActionPanelSwitch,
			Views:       HelpBrowser | HelpDedup,
			Title:       "Switch panel",
			Section:     "Navigation",
			DefaultKeys: []string{"tab"},
			Keywords:    []string{"focus", "toggle panel"},
		},
		{
			ID:           ActionPanelFocusSelections,
			Views:        HelpBrowser,
			Title:        "Focus selections panel",
			Section:      "Navigation",
			DefaultKeys:  []string{"M-s"},
			PreferredKey: "M-s",
			Keywords:     []string{"selections", "strip"},
		},
		{
			ID:           ActionPanelOpenSelectionsRoot,
			Views:        HelpBrowser,
			Title:        "Go to selections common root",
			Section:      "Navigation",
			DefaultKeys:  []string{"C-M-s"},
			PreferredKey: "C-M-s",
			Keywords:     []string{"selections", "root", "common", "navigate"},
		},
		{
			ID:          ActionPanelToggleHideInactive,
			Views:       HelpBrowser,
			Title:       "Hide inactive panel",
			Section:     "Navigation",
			DefaultKeys: []string{"S-tab"},
			Keywords:    []string{"shift-tab", "single panel", "hide panel"},
		},
		{
			ID:          ActionNavUp,
			Views:       helpAllButPreview,
			Title:       "Cursor up",
			Section:     "Navigation",
			DefaultKeys: []string{"up"},
			Keywords:    []string{"previous"},
		},
		{
			ID:          ActionNavDown,
			Views:       helpAllButPreview,
			Title:       "Cursor down",
			Section:     "Navigation",
			DefaultKeys: []string{"down"},
			Keywords:    []string{"next"},
		},
		{
			ID:          ActionNavPageUp,
			Views:       helpAllButPreview,
			Title:       "Page up",
			Section:     "Navigation",
			DefaultKeys: []string{"pgup"},
			Keywords:    []string{"scroll"},
		},
		{
			ID:          ActionNavPageDown,
			Views:       helpAllButPreview,
			Title:       "Page down",
			Section:     "Navigation",
			DefaultKeys: []string{"pgdn"},
			Keywords:    []string{"scroll"},
		},
		{
			ID:          ActionNavTop,
			Views:       helpAllButPreview,
			Title:       "First entry",
			Section:     "Navigation",
			DefaultKeys: []string{"home"},
			Keywords:    []string{"top", "beginning"},
		},
		{
			ID:          ActionNavBottom,
			Views:       helpAllButPreview,
			Title:       "Last entry",
			Section:     "Navigation",
			DefaultKeys: []string{"end"},
			Keywords:    []string{"bottom"},
		},
		{
			ID:           ActionNavOpen,
			Views:        HelpBrowser | HelpCompare | HelpDedup,
			Title:        "Open directory or file",
			Section:      "Navigation",
			DefaultKeys:  []string{"enter", "right"},
			PreferredKey: "Enter",
			Keywords:     []string{"open", "select", "xdg-open", "open file"},
		},
		{
			ID:           ActionNavParent,
			Views:        HelpBrowser,
			Title:        "Parent directory",
			Section:      "Navigation",
			DefaultKeys:  []string{"left", "backspace"},
			PreferredKey: "←",
			Keywords:     []string{"up", "back"},
		},
		{
			ID:           ActionNavHome,
			Views:        HelpBrowser,
			Title:        "Home directory",
			Section:      "Navigation",
			DefaultKeys:  []string{"~", "§"},
			PreferredKey: "~",
			Keywords:     []string{"home", "~", "cd"},
		},
		{
			ID:          ActionNavForward,
			Views:       HelpBrowser,
			Title:       "Forward history",
			Section:     "Navigation",
			DefaultKeys: []string{"M-C-left"},
			Keywords:    []string{"back", "re-enter"},
		},
		{
			ID:          ActionNavBackward,
			Views:       HelpBrowser,
			Title:       "Backward history",
			Section:     "Navigation",
			DefaultKeys: []string{"M-C-right"},
			Keywords:    []string{"forward", "re-enter", "timeline"},
		},
		{
			ID:           ActionPanelHistoryDialog,
			Views:        HelpBrowser,
			Title:        "Directory history",
			Section:      "Navigation",
			DefaultKeys:  []string{"M-h", "C-h"},
			PreferredKey: "M-h",
			Keywords:     []string{"history", "picker", "navigate", "alt-h"},
		},
		{
			ID:           ActionPanelHistoryBothPanels,
			Title:        "Both panels history",
			Section:      "Navigation",
			DefaultKeys:  nil, // overlay: DefaultHistoryDialogOverlayKeys
			PreferredKey: "F5",
			Keywords:     []string{"history dialog", "merge", "both panels", "toggle"},
		},
		{
			ID:           ActionPanelFindDialog,
			Views:        HelpBrowser,
			Title:        "Find files",
			Section:      "Navigation",
			DefaultKeys:  []string{"C-f"},
			PreferredKey: "C-f",
			Keywords:     []string{"find", "search", "recursive", "fuzzy", "locate"},
		},
		{
			ID:           ActionPanelRefresh,
			Views:        HelpBrowser | HelpDedup | HelpFilePreview,
			Title:        "Refresh panel",
			Section:      "Navigation",
			DefaultKeys:  []string{"M-C-r"},
			PreferredKey: "M-C-r",
			Keywords:     []string{"reload"},
		},
		{
			ID:           ActionPanelExternalBrowser,
			Views:        HelpBrowser | HelpJobs | HelpCommands | HelpCompare | HelpFilePreview,
			Title:        "External browser",
			Section:      "Navigation",
			DefaultKeys:  []string{"M-e"},
			PreferredKey: "M-e",
			Keywords:     []string{"xdg-open", "gui", "file manager", "finder"},
		},
		{
			ID:           ActionPanelOpenDirInOther,
			Views:        HelpBrowser,
			Title:        "Open directory in other panel",
			Section:      "Navigation",
			DefaultKeys:  []string{"S-right", "M-o"},
			PreferredKey: "M-o",
			Keywords:     []string{"inactive", "split", "cd", "other panel"},
		},
		{
			ID:           ActionPanelOpenActivePathInOther,
			Views:        HelpBrowser,
			Title:        "Open current path in other panel",
			Section:      "Navigation",
			DefaultKeys:  []string{"S-left", "M-i"},
			PreferredKey: "M-i",
			Keywords:     []string{"inactive", "split", "cd", "cwd", "other panel"},
		},
		{
			ID:           ActionPanelToggleSync,
			Views:        HelpBrowser,
			Title:        "Toggle panel sync",
			Section:      "Navigation",
			DefaultKeys:  []string{"C-M-y"},
			PreferredKey: "C-M-y",
			Keywords:     []string{"sync", "follow", "mirror", "latch", "other panel"},
		},
		{
			ID:           ActionPanelComparePanels,
			Views:        HelpBrowser,
			Title:        "Compare panels",
			Section:      "Navigation",
			DefaultKeys:  []string{"C-M-c"},
			PreferredKey: "C-M-c",
			Keywords:     []string{"diff", "hash", "sync", "mirror", "relocated", "missing"},
		},
		{
			ID:           ActionPanelFindDuplicates,
			Views:        HelpBrowser,
			Title:        "Find duplicates",
			Section:      "Navigation",
			DefaultKeys:  []string{"C-M-u"},
			PreferredKey: "C-M-u",
			Keywords:     []string{"duplicate", "dedup", "dupes", "hash", "identical", "remove"},
		},
		{
			ID:          ActionCompareClose,
			Views:       HelpCompare,
			Title:       "Close compare view",
			Section:     "Navigation",
			DefaultKeys: nil, // overlay: DefaultCompareOverlayKeys
			Keywords:    []string{"back", "browser"},
		},
		{
			ID:          ActionCompareCycleFilter,
			Views:       HelpCompare,
			Title:       "Compare category filter",
			Section:     "Navigation",
			DefaultKeys: nil, // overlay: DefaultCompareOverlayKeys
			Keywords:    []string{"equal", "relocated", "missing", "diff"},
		},
		{
			ID:          ActionCompareResetFilter,
			Views:       HelpCompare,
			Title:       "Reset compare filter to All",
			Section:     "Navigation",
			DefaultKeys: nil, // overlay: DefaultCompareOverlayKeys
			Keywords:    []string{"clear", "all", "show all"},
		},
		{
			ID:          ActionCompareRefresh,
			Views:       HelpCompare,
			Title:       "Refresh compare",
			Section:     "Navigation",
			DefaultKeys: nil, // overlay: DefaultCompareOverlayKeys
			Keywords:    []string{"rescan", "rehash"},
		},
		{
			ID:          ActionCompareMerge,
			Views:       HelpCompare,
			Title:       "Compare merge",
			Section:     "Navigation",
			DefaultKeys: nil, // overlay: DefaultCompareOverlayKeys
			Keywords:    []string{"sync", "reconcile", "copy", "duplicate"},
		},
		{
			ID:          ActionDedupClose,
			Views:       HelpDedup,
			Title:       "Close view",
			Section:     "Find duplicates",
			DefaultKeys: nil, // overlay: DefaultDedupOverlayKeys
			Keywords:    []string{"back", "browser"},
		},
		{
			ID:          ActionDedupToggleSort,
			Views:       HelpDedup,
			Title:       "Toggle sort order",
			Section:     "Find duplicates",
			DefaultKeys: nil, // overlay: DefaultDedupOverlayKeys
			Keywords:    []string{"sort", "path", "wasted", "order"},
		},
		{
			ID:          ActionDedupToggleEmpty,
			Views:       HelpDedup,
			Title:       "Toggle empty files",
			Section:     "Find duplicates",
			DefaultKeys: nil, // overlay: DefaultDedupOverlayKeys
			Keywords:    []string{"empty", "zero", "hide", "ignore"},
		},
		{
			ID:          ActionDedupToggleNode,
			Views:       HelpDedup,
			Title:       "Expand/collapse directory",
			Section:     "Find duplicates",
			DefaultKeys: nil, // overlay: DefaultDedupOverlayKeys
			Keywords:    []string{"tree", "expand", "collapse", "fold", "node"},
		},
		{
			ID:          ActionDedupCollapse,
			Views:       HelpDedup,
			Title:       "Collapse dir / go up",
			Section:     "Find duplicates",
			DefaultKeys: nil, // overlay: DefaultDedupOverlayKeys
			Keywords:    []string{"tree", "collapse", "fold", "parent"},
		},
		{
			ID:          ActionDedupToggleTree,
			Views:       HelpDedup,
			Title:       "Groups / directory view",
			Section:     "Find duplicates",
			DefaultKeys: nil, // overlay: DefaultDedupOverlayKeys
			Keywords:    []string{"tree", "view", "mode", "directory", "group"},
		},
		{
			ID:          ActionDedupCollapseAll,
			Views:       HelpDedup,
			Title:       "Collapse all",
			Section:     "Find duplicates",
			DefaultKeys: nil, // overlay: DefaultDedupOverlayKeys
			Keywords:    []string{"tree", "collapse", "fold", "all"},
		},
		{
			ID:          ActionDedupExpandAll,
			Views:       HelpDedup,
			Title:       "Expand all",
			Section:     "Find duplicates",
			DefaultKeys: nil, // overlay: DefaultDedupOverlayKeys
			Keywords:    []string{"tree", "expand", "unfold", "all"},
		},
		{
			ID:          ActionDedupPrevDir,
			Views:       HelpDedup,
			Title:       "Previous directory",
			Section:     "Find duplicates",
			DefaultKeys: nil, // overlay: DefaultDedupOverlayKeys
			Keywords:    []string{"directory", "folder", "up", "previous", "jump"},
		},
		{
			ID:          ActionDedupNextDir,
			Views:       HelpDedup,
			Title:       "Next directory",
			Section:     "Find duplicates",
			DefaultKeys: nil, // overlay: DefaultDedupOverlayKeys
			Keywords:    []string{"directory", "folder", "down", "next", "jump"},
		},
		{
			ID:          ActionDedupMarkKeep,
			Views:       HelpDedup,
			Title:       "Mark to keep",
			Section:     "Find duplicates",
			DefaultKeys: nil, // overlay: DefaultDedupOverlayKeys
			Keywords:    []string{"keep", "survivor", "duplicate", "dedup"},
		},
		{
			ID:          ActionDedupCompare,
			Views:       HelpDedup,
			Title:       "Compare directories",
			Section:     "Find duplicates",
			DefaultKeys: nil, // overlay: DefaultDedupOverlayKeys
			Keywords:    []string{"compare", "diff", "directory", "merge"},
		},

		// ── Disk usage ──
		{
			ID:          ActionPanelDiskUsageScan,
			Views:       HelpBrowser,
			Title:       "Disk usage scan",
			Section:     "Disk usage",
			DefaultKeys: []string{"C-d"},
			Keywords:    []string{"size", "subtree"},
		},
		{
			ID:          ActionPanelDiskUsageAbortAll,
			Views:       HelpBrowser | HelpFilePreview,
			Title:       "Abort disk usage scans",
			Section:     "Disk usage",
			DefaultKeys: []string{"C-M-d"},
			Keywords:    []string{"cancel", "stop"},
		},
		{
			ID:          ActionPanelDiskUsageClear,
			Views:       HelpBrowser | HelpFilePreview,
			Title:       "Clear disk usage data",
			Section:     "Disk usage",
			DefaultKeys: []string{"M-d"},
			Keywords:    []string{"reset", "forget", "cache"},
		},

		// ── Selection ──
		{
			ID:          ActionPanelSelectToggle,
			Views:       HelpBrowser | HelpCompare | HelpDedup,
			Title:       "Toggle selection",
			Section:     "Selection",
			DefaultKeys: []string{"insert"},
			Keywords:    []string{"select", "mark"},
		},
		{
			ID:          ActionPanelSelectGroup,
			Views:       HelpBrowser,
			Title:       "Select group",
			Section:     "Selection",
			DefaultKeys: []string{"+"},
			Keywords:    []string{"pattern", "glob"},
		},
		{
			ID:          ActionPanelUnselectGroup,
			Views:       HelpBrowser,
			Title:       "Unselect group",
			Section:     "Selection",
			DefaultKeys: []string{"-"},
			Keywords:    []string{"pattern", "glob", "deselect"},
		},
		{
			ID:          ActionPanelInvertSelection,
			Views:       HelpBrowser | HelpDedup,
			Title:       "Invert selection",
			Section:     "Selection",
			DefaultKeys: []string{"*"},
			Keywords:    []string{"reverse"},
		},
		{
			ID:          ActionPanelClearSelection,
			Views:       HelpBrowser | HelpDedup,
			Title:       "Clear selection",
			Section:     "Selection",
			DefaultKeys: []string{"C-u"},
			Keywords:    []string{"deselect", "unmark"},
		},
		{
			ID:           ActionPanelStashToggle,
			Views:        HelpBrowser,
			Title:        "Toggle selection stash",
			Section:      "Selection",
			DefaultKeys:  []string{"M-insert"},
			PreferredKey: "M-insert",
			Keywords:     []string{"stash", "clipboard", "buffer", "selection"},
		},

		// ── Sort & display ──
		{
			ID:          ActionPanelSortDialog,
			Views:       HelpBrowser,
			Title:       "Sort dialog",
			Section:     "Sort & display",
			DefaultKeys: []string{"C-s"},
			Keywords:    []string{"order"},
		},
		{
			ID:          ActionPanelListingFormatDialog,
			Views:       HelpBrowser,
			Title:       "Listing format dialog",
			Section:     "Sort & display",
			DefaultKeys: nil, // opened from Left/Right menu by default
			Keywords:    []string{"columns", "listing", "mtime", "permissions", "brief", "radio"},
		},
		{
			ID:          ActionPanelCycleSort,
			Views:       HelpBrowser,
			Title:       "Cycle sort mode",
			Section:     "Sort & display",
			DefaultKeys: nil, // unbound by default
			Keywords:    []string{"order"},
		},
		{
			ID:          ActionPanelCycleListingFormat,
			Views:       HelpBrowser,
			Title:       "Cycle listing format",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-t"},
			Keywords:    []string{"columns", "listing", "mtime", "permissions", "brief"},
		},
		{
			ID:          ActionPanelToggleCarousel,
			Views:       HelpBrowser,
			Title:       "Toggle carousel view",
			Section:     "Sort & display",
			DefaultKeys: []string{"C-c"},
			Keywords:    []string{"carousel", "columns", "preview", "parent", "child", "godu"},
		},
		{
			ID:          ActionPanelToggleZoomActivePanel,
			Views:       HelpBrowser,
			Title:       "Toggle zoom active panel",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-z"},
			Keywords:    []string{"zoom", "layout", "wide", "column", "split"},
		},
		{
			ID:           ActionPanelToggleSplitOrientation,
			Views:        HelpBrowser,
			Title:        "Toggle split orientation",
			Section:      "Sort & display",
			DefaultKeys:  []string{"C-space"},
			PreferredKey: "C-space",
			Keywords:     []string{"layout", "stacked", "horizontal", "vertical", "split", "orientation"},
		},
		{
			ID:          ActionPanelReverseSort,
			Views:       HelpBrowser,
			Title:       "Reverse sort",
			Section:     "Sort & display",
			DefaultKeys: nil, // unbound by default
			Keywords:    []string{"order", "direction"},
		},
		{
			ID:          ActionPanelToggleHidden,
			Views:       HelpBrowser,
			Title:       "Toggle hidden files",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-."},
			Keywords:    []string{"show", "hide", "dotfiles", "gitignore", "ignored"},
		},
		{
			ID:          ActionPanelMeta,
			Views:       HelpBrowser,
			Title:       "Meta column",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-,"},
			Keywords:    []string{"meta", "column", "command", "count"},
		},
		{
			ID:           ActionPanelMetaEdit,
			Views:        HelpBrowser,
			Title:        "Edit meta commands",
			Section:      "Sort & display",
			DefaultKeys:  []string{"S-M-,", "M-;"},
			PreferredKey: "S-M-,",
			Keywords:     []string{"meta", "meta.toml", "editor", "column", "command"},
		},

		// ── Bookmarks ──
		{
			ID:           ActionBookmarkOpen,
			Views:        HelpBrowser | HelpFilePreview,
			Title:        "Open bookmarks",
			Section:      "Bookmarks",
			DefaultKeys:  []string{"C-g", "C-e"},
			PreferredKey: "C-g",
			Keywords:     []string{"fzf-marks", "marks", "picker", "path-picker", "history", "destination"},
		},
		{
			ID:          ActionBookmarkAdd,
			Views:       HelpBrowser | HelpFilePreview,
			Title:       "Add bookmark",
			Section:     "Bookmarks",
			DefaultKeys: []string{"C-b"},
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
			ID:           ActionBookmarkOpenOther,
			Title:        "Open in other panel",
			Section:      "Bookmarks",
			DefaultKeys:  nil, // overlay: DefaultBookmarkDialogOverlayKeys
			PreferredKey: "M-Enter",
			Keywords:     []string{"bookmark", "inactive panel", "other panel", "open"},
		},
		{
			ID:           ActionFindUnselectAll,
			Title:        "Unselect all",
			Section:      "Find",
			DefaultKeys:  nil, // overlay: DefaultFindDialogOverlayKeys
			PreferredKey: "F4",
			Keywords:     []string{"find dialog", "clear marks", "deselect all"},
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
			ID:           ActionFindSelectGroup,
			Title:        "Select group",
			Section:      "Find",
			DefaultKeys:  nil, // overlay: DefaultFindDialogOverlayKeys
			PreferredKey: "F6",
			Keywords:     []string{"find dialog", "pattern", "glob", "mark"},
		},
		{
			ID:           ActionFindUnselectGroup,
			Title:        "Unselect group",
			Section:      "Find",
			DefaultKeys:  nil, // overlay: DefaultFindDialogOverlayKeys
			PreferredKey: "F7",
			Keywords:     []string{"find dialog", "pattern", "glob", "deselect"},
		},
		{
			ID:           ActionFindOpenInPrimary,
			Title:        "Open in primary panel",
			Section:      "Find",
			DefaultKeys:  nil, // overlay: DefaultFindDialogOverlayKeys
			PreferredKey: "S-left",
			Keywords:     []string{"find dialog", "left panel", "reveal", "cd"},
		},
		{
			ID:           ActionFindOpenInSecondary,
			Title:        "Open in secondary panel",
			Section:      "Find",
			DefaultKeys:  nil, // overlay: DefaultFindDialogOverlayKeys
			PreferredKey: "S-right",
			Keywords:     []string{"find dialog", "right panel", "reveal", "cd"},
		},
		{
			ID:           ActionFlattenDestinationActive,
			Title:        "Flatten destination: active panel",
			Section:      "File operations",
			DefaultKeys:  nil, // overlay: DefaultFlattenDialogOverlayKeys
			PreferredKey: "F5",
			Keywords:     []string{"flatten dialog", "destination", "active panel"},
		},
		{
			ID:           ActionFlattenDestinationInactive,
			Title:        "Flatten destination: inactive panel",
			Section:      "File operations",
			DefaultKeys:  nil, // overlay: DefaultFlattenDialogOverlayKeys
			PreferredKey: "F6",
			Keywords:     []string{"flatten dialog", "destination", "inactive panel", "passive"},
		},
		{
			ID:          ActionRemoteSFTPLink,
			Views:       HelpBrowser,
			Title:       "SFTP ...",
			Section:     "Remote",
			DefaultKeys: []string{"C-r"},
			Keywords:    []string{"ssh", "sftp", "remote", "connect"},
		},

		// ── File operations ──
		{
			ID:           ActionFileRename,
			Views:        HelpBrowser,
			Title:        "Rename / Move",
			Section:      "File operations",
			DefaultKeys:  []string{"S-F6"},
			PreferredKey: "S-F6",
			Keywords:     []string{"rename", "move"},
		},
		{
			ID:          ActionFileMkdir,
			Views:       HelpBrowser,
			Title:       "Create directory",
			Section:     "File operations",
			DefaultKeys: []string{"F7"},
			Keywords:    []string{"mkdir", "folder"},
		},
		{
			ID:           ActionFileMkdirOpenInOther,
			Views:        HelpBrowser,
			Title:        "Create directory and open in other",
			Section:      "File operations",
			DefaultKeys:  []string{"S-F7"},
			PreferredKey: "S-F7",
			Keywords:     []string{"mkdir", "folder", "other", "inactive"},
		},
		{
			ID:          ActionFileDelete,
			Views:       HelpBrowser | HelpDedup,
			Title:       "Delete",
			Section:     "File operations",
			DefaultKeys: []string{"F8", "delete"},
			Keywords:    []string{"remove", "trash"},
		},
		{
			ID:          ActionFileChmod,
			Views:       HelpBrowser,
			Title:       "Change mode",
			Section:     "File operations",
			DefaultKeys: nil, // unbound by default (menu only)
			Keywords:    []string{"permissions", "chmod"},
		},
		{
			ID:          ActionFileChown,
			Views:       HelpBrowser,
			Title:       "Change owner",
			Section:     "File operations",
			DefaultKeys: nil, // unbound by default (menu only)
			Keywords:    []string{"owner", "group", "chown"},
		},
		{
			ID:          ActionFileSymlink,
			Views:       HelpBrowser,
			Title:       "Symlink",
			Section:     "File operations",
			DefaultKeys: nil, // unbound by default (menu only)
			Keywords:    []string{"link", "symbolic"},
		},
		{
			ID:          ActionFileHardlink,
			Views:       HelpBrowser,
			Title:       "Hard link",
			Section:     "File operations",
			DefaultKeys: nil, // unbound by default (menu only)
			Keywords:    []string{"link"},
		},
		{
			ID:          ActionFileView,
			Views:       HelpBrowser,
			Title:       "Full screen file view",
			Section:     "File operations",
			DefaultKeys: []string{"F3"},
			Keywords:    []string{"viewer", "view", "fullscreen", "bat"},
		},
		{
			ID:          ActionFileViewThemePicker,
			Views:       HelpFilePreview,
			Title:       "Theme picker in file view",
			Section:     "File operations",
			DefaultKeys: nil, // overlay: DefaultFilePreviewOverlayKeys
			Keywords:    []string{"theme", "preview", "view", "f9"},
		},
		{
			ID:          ActionFileViewDiffNextHunk,
			Views:       HelpFilePreview,
			Title:       "Next diff hunk",
			Section:     "File operations",
			DefaultKeys: nil, // overlay: DefaultFilePreviewOverlayKeys
			Keywords:    []string{"diff", "hunk", "change", "jump"},
		},
		{
			ID:          ActionFileViewDiffPrevHunk,
			Views:       HelpFilePreview,
			Title:       "Previous diff hunk",
			Section:     "File operations",
			DefaultKeys: nil, // overlay: DefaultFilePreviewOverlayKeys
			Keywords:    []string{"diff", "hunk", "change", "jump"},
		},
		{
			ID:          ActionFileQuickView,
			Views:       HelpBrowser,
			Title:       "Quick view",
			Section:     "File operations",
			DefaultKeys: []string{"S-F3"},
			Keywords:    []string{"preview", "inactive", "bat"},
		},
		{
			ID:           ActionFileQuickViewPreviewPageUp,
			Views:        HelpBrowser | HelpFilePreview,
			Title:        "Quick view preview page up",
			Section:      "File operations",
			DefaultKeys:  []string{"C-k"},
			PreferredKey: "C-k",
			Keywords:     []string{"preview", "inactive", "scroll", "page"},
		},
		{
			ID:           ActionFileQuickViewPreviewPageDown,
			Views:        HelpBrowser | HelpFilePreview,
			Title:        "Quick view preview page down",
			Section:      "File operations",
			DefaultKeys:  []string{"C-j"},
			PreferredKey: "C-j",
			Keywords:     []string{"preview", "inactive", "scroll", "page"},
		},
		{
			ID:          ActionFileEdit,
			Views:       HelpBrowser | HelpFilePreview,
			Title:       "Edit file",
			Section:     "File operations",
			DefaultKeys: []string{"F4"},
			Keywords:    []string{"editor", "edit"},
		},
		{
			ID:          ActionMenuFileChattr,
			Views:       HelpBrowser,
			Title:       "Change file attributes",
			Section:     "File operations",
			DefaultKeys: nil,
			Keywords:    []string{"chattr", "menu"},
		},
		{
			ID:          ActionCopy,
			Views:       HelpBrowser,
			Title:       "Copy",
			Section:     "File operations",
			DefaultKeys: []string{"F5"},
			Keywords:    []string{"duplicate"},
		},
		{
			ID:           ActionFileCopyHere,
			Views:        HelpBrowser,
			Title:        "Duplicate",
			Section:      "File operations",
			DefaultKeys:  []string{"S-F5"},
			PreferredKey: "S-F5",
			Keywords:     []string{"copy here", "directory", "same directory"},
		},
		{
			ID:          ActionFileExtract,
			Views:       HelpBrowser,
			Title:       "Extract archives",
			Section:     "File operations",
			DefaultKeys: nil,
			Keywords:    []string{"unpack", "unzip", "tar", "archive", "decompress"},
		},
		{
			ID:          ActionMove,
			Views:       HelpBrowser,
			Title:       "Move",
			Section:     "File operations",
			DefaultKeys: []string{"F6"},
			Keywords:    []string{"move"},
		},
		{
			ID:           ActionFileFlatten,
			Views:        HelpBrowser,
			Title:        "Flatten directories",
			Section:      "File operations",
			DefaultKeys:  []string{"C-M-f"},
			PreferredKey: "C-M-f",
			Keywords:     []string{"flatten", "hoist", "directory"},
		},

		// ── Commands ──
		{
			ID:           ActionCommandsOpen,
			Views:        helpAllViews,
			Title:        "Open Commands view",
			Section:      "Commands",
			DefaultKeys:  []string{"M-c"},
			PreferredKey: "M-c",
			Keywords:     []string{"shell", "stdout", "stderr"},
		},
		{
			ID:          ActionCommandsClose,
			Views:       HelpCommands,
			Title:       "Close Commands view",
			Section:     "Commands",
			DefaultKeys: nil, // overlay: DefaultCommandsOverlayKeys
			Keywords:    []string{"back", "browser", "esc"},
		},
		{
			ID:           ActionCommandsTerminate,
			Views:        HelpCommands,
			Title:        "Terminate command",
			Section:      "Commands",
			DefaultKeys:  nil, // overlay: DefaultCommandsOverlayKeys
			PreferredKey: "F8",
			Keywords:     []string{"sigterm", "stop", "process"},
		},
		{
			ID:           ActionCommandsKill,
			Views:        HelpCommands,
			Title:        "Kill command",
			Section:      "Commands",
			DefaultKeys:  nil, // overlay: DefaultCommandsOverlayKeys
			PreferredKey: "S-F8",
			Keywords:     []string{"sigkill", "force", "stop", "process"},
		},
		{
			ID:          ActionFileRunForEach,
			Views:       HelpBrowser,
			Title:       "Run for each",
			Section:     "Commands",
			DefaultKeys: nil,
			Keywords:    []string{"exec", "subprocess", "batch"},
		},

		// ── Messages (status / toast log) ──
		{
			ID:           ActionMessagesOpen,
			Views:        helpAllViews,
			Title:        "Open Messages view",
			Section:      "Messages",
			DefaultKeys:  []string{"M-m"},
			PreferredKey: "M-m",
			Keywords:     []string{"log", "toast", "status", "banner"},
		},
		{
			ID:          ActionMessagesClose,
			Views:       HelpMessages,
			Title:       "Close Messages view",
			Section:     "Messages",
			DefaultKeys: nil, // overlay: DefaultMessagesOverlayKeys
			Keywords:    []string{"back", "browser", "esc"},
		},
		{
			ID:          ActionMessagesClear,
			Views:       HelpMessages,
			Title:       "Clear messages",
			Section:     "Messages",
			DefaultKeys: nil,
			Keywords:    []string{"log", "toast", "status"},
		},

		// ── Jobs ──
		// jobs.open defaults belong in [main] (global). Other
		// jobs.* defaults live in DefaultJobsOverlayKeys ([jobs]).
		{
			ID:           ActionJobsOpen,
			Views:        helpAllViews,
			Title:        "Open jobs view",
			Section:      "Jobs",
			DefaultKeys:  []string{"M-j"},
			PreferredKey: "M-j",
			Keywords:     []string{"queue", "background"},
		},
		{
			ID:           ActionJobsAnswerBlocker,
			Views:        HelpBrowser | HelpJobs,
			Title:        "Answer job blocker",
			Section:      "Jobs",
			DefaultKeys:  []string{"C-q", "M-q"},
			PreferredKey: "C-q",
			Keywords:     []string{"blocker", "waiting", "conflict"},
		},
		{
			ID:          ActionJobsCancel,
			Views:       HelpJobs,
			Title:       "Cancel job",
			Section:     "Jobs",
			DefaultKeys: nil,
			Keywords:    []string{"abort"},
		},
		{
			ID:          ActionJobsPause,
			Views:       HelpJobs,
			Title:       "Pause queued job",
			Section:     "Jobs",
			DefaultKeys: nil,
			Keywords:    []string{"hold"},
		},
		{
			ID:          ActionJobsResume,
			Views:       HelpJobs,
			Title:       "Resume paused job",
			Section:     "Jobs",
			DefaultKeys: nil,
			Keywords:    []string{"unpause", "start"},
		},
		{
			ID:          ActionJobsQueueUp,
			Views:       HelpJobs,
			Title:       "Move job up in queue",
			Section:     "Jobs",
			DefaultKeys: nil,
			Keywords:    []string{"reorder"},
		},
		{
			ID:          ActionJobsQueueDown,
			Views:       HelpJobs,
			Title:       "Move job down in queue",
			Section:     "Jobs",
			DefaultKeys: nil,
			Keywords:    []string{"reorder"},
		},
		{
			ID:          ActionJobsClose,
			Views:       HelpJobs,
			Title:       "Close jobs view",
			Section:     "Jobs",
			DefaultKeys: nil, // overlay: DefaultJobsOverlayKeys
			Keywords:    []string{"back", "browser", "esc"},
		},
		{
			ID:          ActionJobsClearFinished,
			Views:       HelpJobs,
			Title:       "Clear finished jobs",
			Section:     "Jobs",
			DefaultKeys: nil,
			Keywords:    []string{"remove", "done"},
		},

		// ── Options dialogs ──
		{
			ID:          ActionUIOpenTheme,
			Views:       HelpBrowser,
			Title:       "Theme picker",
			Section:     "UI",
			DefaultKeys: nil,
			Keywords:    []string{"appearance", "colors"},
		},
		{
			ID:          ActionUICalibrateDebounce,
			Views:       HelpBrowser | HelpFilePreview,
			Title:       "Calibrate debounce",
			Section:     "UI",
			DefaultKeys: nil,
			Keywords:    []string{"keyboard", "repeat", "debounce"},
		},
		{
			ID:          ActionUIOpenConfig,
			Views:       HelpBrowser | HelpFilePreview,
			Title:       "Configuration editor",
			Section:     "UI",
			DefaultKeys: nil,
			Keywords:    []string{"settings", "toml"},
		},
		{
			ID:           ActionDialogInputRestoreDefault,
			Title:        "Restore default placeholder",
			Section:      "UI",
			DefaultKeys:  nil, // bound only via [dialog.input] (defaults C-r and C-d)
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
			Views:       HelpBrowser,
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
