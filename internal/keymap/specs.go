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
	// LeaderKey is the activation character for the Esc function menu (empty = excluded).
	// Lowercase and uppercase are distinct (f vs F). Use lowercase for the frequent action
	// and uppercase for a related secondary action on the same letter. Non-letters (e.g. ?) allowed.
	LeaderKey string
	// CopyMenuKey is the activation character for the `"` copy menu (empty = excluded).
	CopyMenuKey string
	// PreviewMenuKey is the activation character for the `:` fullscreen-preview menu
	// (empty = excluded). Letters only; case-sensitive (r vs R).
	PreviewMenuKey string
}

// DefaultActionSpecs returns all known configurable actions in display order.
func DefaultActionSpecs() []ActionSpec {
	return []ActionSpec{
		// ── App ──
		{
			ID:             ActionAppQuit,
			Views:          helpAllViews,
			Title:          "Quit",
			Section:        "App",
			DefaultKeys:    []string{"F10"},
			PreferredKey:   "F10",
			Keywords:       []string{"exit", "close"},
			LeaderKey:      "q",
			PreviewMenuKey: "q",
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
			DefaultKeys:  []string{"F1", "?"},
			PreferredKey: "F1",
			Keywords:     []string{"help screen"},
			LeaderKey:    "?",
		},
		{
			ID:           ActionAppUserMenu,
			Views:        helpAllButPreview,
			Title:        "User menu",
			Section:      "App",
			DefaultKeys:  []string{"F2"},
			PreferredKey: "F2",
			Keywords:     []string{"menu.toml", "custom commands"},
		},
		{
			ID:           ActionAppLeaderMenu,
			Views:        HelpBrowser,
			Title:        "Function menu",
			Section:      "App",
			DefaultKeys:  []string{":"},
			PreferredKey: ":",
			Keywords:     []string{"function menu", "colon", "shortcuts"},
		},
		{
			ID:           ActionAppCopyMenu,
			Views:        HelpBrowser,
			Title:        "Copy to clipboard menu",
			Section:      "Clipboard",
			DefaultKeys:  []string{"\""},
			PreferredKey: "\"",
			Keywords:     []string{"copy to clipboard menu", "copy menu", "clipboard", "quote"},
		},
		{
			ID:          ActionClipboardCopyFileURL,
			Views:       HelpBrowser,
			Title:       "Copy file URL",
			Section:     "Clipboard",
			Keywords:    []string{"clipboard", "path", "url", "copy"},
			CopyMenuKey: "c",
		},
		{
			ID:          ActionClipboardCopyDirURL,
			Views:       HelpBrowser,
			Title:       "Copy directory URL",
			Section:     "Clipboard",
			Keywords:    []string{"clipboard", "directory", "dirname", "url", "copy"},
			CopyMenuKey: "d",
		},
		{
			ID:          ActionClipboardCopyFilename,
			Views:       HelpBrowser,
			Title:       "Copy filename",
			Section:     "Clipboard",
			Keywords:    []string{"clipboard", "basename", "name", "copy"},
			CopyMenuKey: "f",
		},
		{
			ID:          ActionClipboardCopyFilenameWithoutExt,
			Views:       HelpBrowser,
			Title:       "Copy filename without extension",
			Section:     "Clipboard",
			Keywords:    []string{"clipboard", "stem", "name", "copy"},
			CopyMenuKey: "n",
		},
		{
			ID:           ActionAppUserMenuEdit,
			Views:        helpAllButPreview,
			Title:        "Edit user menu",
			Section:      "App",
			DefaultKeys:  []string{"S-F2"},
			PreferredKey: "S-F2",
			Keywords:     []string{"menu.toml", "editor", "custom commands"},
		},
		{
			ID:           ActionAppDropToShell,
			Views:        HelpBrowser,
			Title:        "Open shell",
			Section:      "Shell",
			DefaultKeys:  []string{"C-o"},
			PreferredKey: "C-o",
			Keywords:     []string{"shell", "subshell", "ctrl-o", "terminal"},
			LeaderKey:    "O",
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
			DefaultKeys: []string{"C-right"},
			Keywords:    []string{"back", "re-enter"},
		},
		{
			ID:          ActionNavBackward,
			Views:       HelpBrowser,
			Title:       "Backward history",
			Section:     "Navigation",
			DefaultKeys: []string{"C-left"},
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
			LeaderKey:    "h",
		},
		{
			ID:           ActionPanelGitFilterMenu,
			Views:        HelpBrowser,
			Title:        "Git filter menu",
			Section:      "Navigation",
			DefaultKeys:  []string{"M-g"},
			PreferredKey: "M-g",
			Keywords:     []string{"git", "filter", "staged", "unstaged", "tracked", "untracked"},
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
			LeaderKey:    "f",
		},
		{
			ID:           ActionPanelRefresh,
			Views:        HelpBrowser | HelpDedup,
			Title:        "Refresh panel",
			Section:      "Navigation",
			DefaultKeys:  []string{"C-n"},
			PreferredKey: "C-n",
			Keywords:     []string{"reload"},
			LeaderKey:    "n",
		},
		{
			ID:           ActionPanelExternalBrowser,
			Views:        HelpBrowser | HelpJobs | HelpCommands | HelpCompare | HelpFilePreview,
			Title:        "External browser",
			Section:      "Navigation",
			DefaultKeys:  []string{"M-x"},
			PreferredKey: "M-x",
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
			DefaultKeys:  []string{"M-y"},
			PreferredKey: "M-y",
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
			LeaderKey:    "C",
		},
		{
			ID:           ActionPanelFindDuplicates,
			Views:        HelpBrowser,
			Title:        "Find duplicates",
			Section:      "Navigation",
			DefaultKeys:  []string{"C-M-f"},
			PreferredKey: "C-M-f",
			Keywords:     []string{"duplicate", "dedup", "dupes", "hash", "identical", "remove"},
			LeaderKey:    "P",
		},
		{
			ID:          ActionPanelFilterDialog,
			Views:       HelpBrowser,
			Title:       "Filter",
			Section:     "Filter",
			DefaultKeys: []string{"M-f"},
			Keywords:    []string{"regex", "glob", "pattern", "narrow", "hide", "shell"},
			LeaderKey:   "F",
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
			DefaultKeys: []string{"M-d"},
			Keywords:    []string{"size", "subtree"},
			LeaderKey:   "D",
		},
		{
			ID:          ActionPanelDiskUsageClear,
			Views:       HelpBrowser,
			Title:       "Abort and clear disk usage",
			Section:     "Disk usage",
			DefaultKeys: []string{"M-S-d"},
			Keywords:    []string{"abort", "cancel", "stop", "reset", "forget", "cache"},
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
			LeaderKey:   "g",
		},
		{
			ID:          ActionPanelUnselectGroup,
			Views:       HelpBrowser,
			Title:       "Unselect group",
			Section:     "Selection",
			DefaultKeys: []string{"-", "M-u"},
			Keywords:    []string{"pattern", "glob", "deselect"},
			LeaderKey:   "u",
		},
		{
			ID:          ActionPanelInvertSelection,
			Views:       HelpBrowser | HelpDedup,
			Title:       "Invert selection",
			Section:     "Selection",
			DefaultKeys: []string{"*", "C-i"},
			Keywords:    []string{"reverse"},
			LeaderKey:   "i",
		},
		{
			ID:          ActionPanelClearSelection,
			Views:       HelpBrowser | HelpDedup,
			Title:       "Unselect all",
			Section:     "Selection",
			DefaultKeys: []string{"C-u"},
			Keywords:    []string{"deselect", "unmark", "clear"},
			LeaderKey:   "U",
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
			ID:           ActionPanelSortDialog,
			Views:        HelpBrowser,
			Title:        "Sort dialog",
			Section:      "Sort & display",
			DefaultKeys:  []string{"C-s"},
			PreferredKey: "C-s",
			Keywords:     []string{"order"},
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
			DefaultKeys: []string{"M-v"},
			Keywords:    []string{"carousel", "columns", "preview", "parent", "child", "godu"},
		},
		{
			ID:          ActionPanelToggleTree,
			Views:       HelpBrowser,
			Title:       "Toggle tree view / expand directory",
			Section:     "Sort & display",
			DefaultKeys: []string{"space"},
			Keywords:    []string{"tree", "expand", "collapse", "directory", "nested"},
		},
		{
			ID:          ActionPanelTreeExpand,
			Views:       HelpBrowser,
			Title:       "Expand tree row",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-right"},
			Keywords:    []string{"tree", "expand", "directory", "nested"},
		},
		{
			ID:          ActionPanelTreeCollapse,
			Views:       HelpBrowser,
			Title:       "Collapse tree row",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-left"},
			Keywords:    []string{"tree", "collapse", "directory", "nested"},
		},
		{
			ID:          ActionPanelTreeCollapseAll,
			Views:       HelpBrowser,
			Title:       "Collapse all one level (tree)",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-C-left"},
			Keywords:    []string{"tree", "collapse", "all", "shallow", "directory", "nested"},
		},
		{
			ID:          ActionPanelTreeCollapseAllFull,
			Views:       HelpBrowser,
			Title:       "Collapse all (tree)",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-S-left"},
			Keywords:    []string{"tree", "collapse", "all", "full", "directory", "nested"},
		},
		{
			ID:          ActionPanelTreeExpandAllShallow,
			Views:       HelpBrowser,
			Title:       "Expand all one level (tree)",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-C-right"},
			Keywords:    []string{"tree", "expand", "all", "shallow", "directory", "nested"},
		},
		{
			ID:          ActionPanelTreeExpandAllFull,
			Views:       HelpBrowser,
			Title:       "Expand all to max depth (tree)",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-S-right"},
			Keywords:    []string{"tree", "expand", "all", "full", "max", "depth", "directory", "nested"},
		},
		{
			ID:          ActionPanelTreePrevSiblingDir,
			Views:       HelpBrowser,
			Title:       "Previous sibling directory (tree)",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-up"},
			Keywords:    []string{"tree", "sibling", "directory", "previous", "jump"},
		},
		{
			ID:          ActionPanelTreeNextSiblingDir,
			Views:       HelpBrowser,
			Title:       "Next sibling directory (tree)",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-down"},
			Keywords:    []string{"tree", "sibling", "directory", "next", "jump"},
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
			LeaderKey:   ".",
		},
		{
			ID:          ActionPanelMeta,
			Views:       HelpBrowser,
			Title:       "Meta column",
			Section:     "Sort & display",
			DefaultKeys: []string{"M-,"},
			Keywords:    []string{"meta", "column", "command", "count"},
			LeaderKey:   ",",
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
			Views:        HelpBrowser,
			Title:        "Open bookmarks",
			Section:      "Bookmarks",
			DefaultKeys:  []string{"C-b", "C-g", "C-e"},
			PreferredKey: "C-b",
			Keywords:     []string{"fzf-marks", "marks", "picker", "path-picker", "history", "destination"},
			LeaderKey:    "b",
		},
		{
			ID:          ActionBookmarkAdd,
			Views:       HelpBrowser,
			Title:       "Add bookmark",
			Section:     "Bookmarks",
			DefaultKeys: []string{"C-M-b"},
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
			ID:           ActionFindView,
			Title:        "View",
			Section:      "Find",
			DefaultKeys:  nil, // overlay: DefaultFindDialogOverlayKeys
			PreferredKey: "F3",
			Keywords:     []string{"find dialog", "preview", "quick view", "fullscreen"},
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
			ID:           ActionDestinationActivePanel,
			Title:        "Set destination to active panel path",
			Section:      "File operations",
			DefaultKeys:  nil, // overlay: shared by DefaultFlattenDialogOverlayKeys / DefaultTransferDialogOverlayKeys
			PreferredKey: "S-left",
			Keywords:     []string{"flatten dialog", "copy dialog", "move dialog", "destination", "active panel"},
		},
		{
			ID:           ActionDestinationInactivePanel,
			Title:        "Set destination to inactive panel path",
			Section:      "File operations",
			DefaultKeys:  nil, // overlay: shared by DefaultFlattenDialogOverlayKeys / DefaultTransferDialogOverlayKeys
			PreferredKey: "S-right",
			Keywords:     []string{"flatten dialog", "copy dialog", "move dialog", "destination", "inactive panel", "passive"},
		},
		{
			ID:          ActionRemoteSFTPLink,
			Views:       HelpBrowser,
			Title:       "SFTP ...",
			Section:     "Remote",
			DefaultKeys: []string{"M-r"},
			Keywords:    []string{"ssh", "sftp", "remote", "connect"},
		},

		// ── File operations ──
		{
			ID:           ActionFileRename,
			Views:        HelpBrowser,
			Title:        "Rename",
			Section:      "File operations",
			DefaultKeys:  []string{"S-F6", "C-r"},
			PreferredKey: "S-F6",
			Keywords:     []string{"rename"},
			LeaderKey:    "r",
		},
		{
			ID:           ActionFileMkdir,
			Views:        HelpBrowser,
			Title:        "Create directory",
			Section:      "File operations",
			DefaultKeys:  []string{"F7", "C-m"},
			PreferredKey: "F7",
			Keywords:     []string{"mkdir", "folder"},
			LeaderKey:    "M",
		},
		{
			ID:           ActionFileMkdirOpenInOther,
			Views:        HelpBrowser,
			Title:        "Create directory + open",
			Section:      "File operations",
			DefaultKeys:  []string{"S-F7"},
			PreferredKey: "S-F7",
			Keywords:     []string{"mkdir", "folder", "other", "inactive"},
			LeaderKey:    "o",
		},
		{
			ID:             ActionFileDelete,
			Views:          HelpBrowser | HelpDedup,
			Title:          "Delete",
			Section:        "File operations",
			DefaultKeys:    []string{"F8", "delete", "C-d"},
			PreferredKey:   "F8",
			Keywords:       []string{"remove", "trash"},
			LeaderKey:      "d",
			PreviewMenuKey: "d",
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
			ID:           ActionFileView,
			Views:        HelpBrowser,
			Title:        "Full screen file view",
			Section:      "File operations",
			DefaultKeys:  []string{"F3", "C-v"},
			PreferredKey: "F3",
			Keywords:     []string{"viewer", "view", "fullscreen", "bat"},
			LeaderKey:    "v",
		},
		{
			ID:          ActionFileViewMenu,
			Views:       HelpFilePreview,
			Title:       "Preview menu",
			Section:     "Preview",
			DefaultKeys: nil, // overlay: DefaultFilePreviewOverlayKeys
			Keywords:    []string{"preview menu", "colon", "menu"},
		},
		{
			ID:             ActionFileViewThemePicker,
			Views:          HelpFilePreview,
			Title:          "Theme picker in file view",
			Section:        "Preview",
			DefaultKeys:    nil, // overlay: DefaultFilePreviewOverlayKeys
			Keywords:       []string{"theme", "preview", "view", "f9"},
			PreviewMenuKey: "t",
		},
		{
			ID:             ActionFileViewToggleRaw,
			Views:          HelpFilePreview,
			Title:          "Toggle raw/rendered markdown",
			Section:        "Preview",
			DefaultKeys:    nil, // overlay: DefaultFilePreviewOverlayKeys
			Keywords:       []string{"markdown", "raw", "render", "source", "f6"},
			PreviewMenuKey: "r",
		},
		{
			ID:             ActionFileViewReload,
			Views:          HelpFilePreview,
			Title:          "Reload file view",
			Section:        "Preview",
			DefaultKeys:    nil, // overlay: DefaultFilePreviewOverlayKeys
			Keywords:       []string{"reload", "refresh", "f5"},
			PreviewMenuKey: "R",
		},
		{
			ID:             ActionFileViewDiffNextHunk,
			Views:          HelpBrowser | HelpFilePreview,
			Title:          "Next diff change",
			Section:        "Preview",
			DefaultKeys:    []string{"C-M-j"},
			Keywords:       []string{"diff", "hunk", "change", "chunk", "jump"},
			PreviewMenuKey: "n",
		},
		{
			ID:             ActionFileViewDiffPrevHunk,
			Views:          HelpBrowser | HelpFilePreview,
			Title:          "Previous diff change",
			Section:        "Preview",
			DefaultKeys:    []string{"C-M-k"},
			Keywords:       []string{"diff", "hunk", "change", "chunk", "jump"},
			PreviewMenuKey: "p",
		},
		{
			ID:             ActionFileViewSearchStart,
			Views:          HelpFilePreview,
			Title:          "Search in file view",
			Section:        "Preview",
			DefaultKeys:    nil, // overlay: DefaultFilePreviewOverlayKeys
			Keywords:       []string{"search", "find", "incremental"},
			PreviewMenuKey: "s",
		},
		{
			ID:          ActionFileViewClose,
			Views:       HelpFilePreview,
			Title:       "Close file view",
			Section:     "Preview",
			DefaultKeys: nil, // overlay: DefaultFilePreviewOverlayKeys
			Keywords:    []string{"close", "quit", "back", "browser", "esc"},
		},
		{
			ID:          ActionFileViewSearchNext,
			Views:       HelpFilePreview,
			Title:       "Next search match",
			Section:     "Preview",
			DefaultKeys: nil, // overlay: DefaultFilePreviewOverlayKeys
			Keywords:    []string{"search", "find", "next", "match"},
		},
		{
			ID:          ActionFileViewSearchPrev,
			Views:       HelpFilePreview,
			Title:       "Previous search match",
			Section:     "Preview",
			DefaultKeys: nil, // overlay: DefaultFilePreviewOverlayKeys
			Keywords:    []string{"search", "find", "previous", "match"},
		},
		{
			ID:           ActionFileQuickView,
			Views:        HelpBrowser,
			Title:        "Quick view",
			Section:      "File operations",
			DefaultKeys:  []string{"S-F3", "C-M-v"},
			PreferredKey: "S-F3",
			Keywords:     []string{"preview", "inactive", "bat"},
			LeaderKey:    "V",
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
			ID:             ActionFileEdit,
			Views:          HelpBrowser | HelpFilePreview,
			Title:          "Edit file",
			Section:        "File operations",
			DefaultKeys:    []string{"F4", "M-e"},
			PreferredKey:   "F4",
			Keywords:       []string{"editor", "edit"},
			LeaderKey:      "e",
			PreviewMenuKey: "e",
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
			ID:           ActionCopy,
			Views:        HelpBrowser,
			Title:        "Copy",
			Section:      "File operations",
			DefaultKeys:  []string{"F5", "C-c"},
			PreferredKey: "F5",
			Keywords:     []string{"duplicate"},
			LeaderKey:    "c",
		},
		{
			ID:           ActionFileDuplicate,
			Views:        HelpBrowser,
			Title:        "Duplicate",
			Section:      "File operations",
			DefaultKeys:  []string{"S-F5", "C-p"},
			PreferredKey: "S-F5",
			Keywords:     []string{"copy here", "directory", "same directory"},
			LeaderKey:    "p",
		},
		{
			ID:          ActionFileExtract,
			Views:       HelpBrowser,
			Title:       "Extract archives",
			Section:     "File operations",
			DefaultKeys: []string{"C-x"},
			Keywords:    []string{"unpack", "unzip", "tar", "archive", "decompress"},
			LeaderKey:   "x",
		},
		{
			ID:           ActionMove,
			Views:        HelpBrowser,
			Title:        "Move",
			Section:      "File operations",
			DefaultKeys:  []string{"F6", "C-M-m"},
			PreferredKey: "F6",
			Keywords:     []string{"move"},
			LeaderKey:    "m",
		},
		{
			ID:           ActionFileFlatten,
			Views:        HelpBrowser,
			Title:        "Flatten directories",
			Section:      "File operations",
			DefaultKeys:  []string{"C-l"},
			PreferredKey: "C-l",
			Keywords:     []string{"flatten", "hoist", "directory"},
			LeaderKey:    "l",
		},

		// ── Commands ──
		{
			ID:           ActionCommandsOpen,
			Views:        helpAllViews,
			Title:        "Open Commands view",
			Section:      "Commands",
			DefaultKeys:  []string{"C-M-e"},
			PreferredKey: "C-M-e",
			Keywords:     []string{"shell", "stdout", "stderr"},
			LeaderKey:    "E",
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
			ID:           ActionFileRunForEach,
			Views:        HelpBrowser,
			Title:        "Run for each",
			Section:      "Commands",
			DefaultKeys:  []string{"C-M-r"},
			PreferredKey: "C-M-r",
			Keywords:     []string{"exec", "subprocess", "batch"},
			LeaderKey:    "R",
		},

		// ── Messages (status / toast log) ──
		{
			ID:           ActionMessagesOpen,
			Views:        helpAllViews,
			Title:        "Open Messages view",
			Section:      "Messages",
			DefaultKeys:  []string{"M-l"},
			PreferredKey: "M-l",
			Keywords:     []string{"log", "toast", "status", "banner"},
			LeaderKey:    "L",
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
			LeaderKey:    "j",
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
			Views:       HelpBrowser,
			Title:       "Calibrate debounce",
			Section:     "UI",
			DefaultKeys: nil,
			Keywords:    []string{"keyboard", "repeat", "debounce"},
		},
		{
			ID:          ActionUIOpenConfig,
			Views:       HelpBrowser,
			Title:       "Configuration editor",
			Section:     "UI",
			DefaultKeys: nil,
			Keywords:    []string{"settings", "toml"},
		},
		{
			ID:          ActionPreviewImageCapabilityDialog,
			Views:       HelpBrowser | HelpFilePreview,
			Title:       "Image terminal capabilities",
			Section:     "Preview",
			DefaultKeys: []string{"M-F3"},
			Keywords:    []string{"sixel", "kitty", "placeholder", "wezterm", "graphics", "image"},
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
