package menu

import "github.com/paranoidi/paras-commander/internal/keymap"

// State is the renderable state for the top pulldown menu.
type State struct {
	Open         bool
	PulldownOpen bool
	ActiveMenu   int
	SelectedItem int
}

// Item is a single selectable pulldown entry.
type Item struct {
	// Action is the stable routing ID (typically a keymap.Action* constant).
	// Empty when Separator is true.
	Action string
	Label  string
	// Shortcut is the letter highlighted when the pulldown is open (plain key).
	Shortcut rune
	// KeyLabel is shown on the right side of a pulldown row and used for footer/F-key routing.
	// BrowserDefinitions / JobsDefinitions fill this from the resolved keymap.
	KeyLabel  string
	Separator bool
}

// Definition describes one top-level pulldown menu.
type Definition struct {
	ID         TopID
	PanelScope int // PanelScopeNone, or PanelScopePrimary / PanelScopeSecondary
	Label      string
	Shortcut   rune
	Items      []Item
}

// ActiveDefinitions returns defs if non-empty, otherwise the built-in defaults.
func ActiveDefinitions(defs []Definition) []Definition {
	if len(defs) > 0 {
		return defs
	}
	return Definitions()
}

// ApplyOverlayMenuKeyLabels sets Item.KeyLabel for every non-separator item, preferring the
// overlay's chords before global. Pass a nil overlay for global-only menus (e.g. the browser).
func ApplyOverlayMenuKeyLabels(defs []Definition, global, overlay *keymap.Map) {
	for i := range defs {
		for j := range defs[i].Items {
			item := &defs[i].Items[j]
			if item.Separator || item.Action == "" {
				continue
			}
			item.KeyLabel = keymap.MenuBindingLabelPreferOverlay(global, overlay, item.Action)
		}
	}
}

// BrowserDefinitions returns Definitions() with KeyLabels resolved from km.
// When dev is true, the Dev pulldown is appended (see DevDefinition).
func BrowserDefinitions(km *keymap.Map, dev bool) []Definition {
	defs := Definitions()
	if dev {
		defs = append(defs, DevDefinition())
	}
	ApplyOverlayMenuKeyLabels(defs, km, nil)
	return defs
}

// Definitions returns the built-in v1 menu tree.
func Definitions() []Definition {
	optionsItems := []Item{
		{Action: keymap.ActionUIOpenConfig, Label: "Configuration", Shortcut: 'c'},
		{Action: keymap.ActionPreviewImageCapabilityDialog, Label: "Configure graphics", Shortcut: 'g'},
		{Action: keymap.ActionUICalibrateDebounce, Label: "Calibrate Debounce", Shortcut: 'd'},
		{Action: keymap.ActionUIOpenTheme, Label: "Theme", Shortcut: 't'},
	}

	return []Definition{
		{
			ID:         TopPanelLeft,
			PanelScope: PanelScopePrimary,
			Label:      "Left",
			Shortcut:   'l',
			Items: []Item{
				{Action: keymap.ActionFileQuickView, Label: "Quick view", Shortcut: 'q'},
				{Action: keymap.ActionPanelSortDialog, Label: "Sort...", Shortcut: 's'},
				{Action: keymap.ActionPanelToggleHidden, Label: "Toggle hidden", Shortcut: 'h'},
				{Action: keymap.ActionPanelRefresh, Label: "Refresh", Shortcut: 'r'},
				{Action: keymap.ActionPanelDiskUsageScan, Label: "Disk usage", Shortcut: 'u'},
				{Action: keymap.ActionPanelHistoryDialog, Label: "History...", Shortcut: 'y'},
				{Action: keymap.ActionPanelFindDialog, Label: "Find ...", Shortcut: 'i'},
				{Action: keymap.ActionPanelExternalBrowser, Label: "External browser", Shortcut: 'e'},
				{Action: keymap.ActionPanelMeta, Label: "Meta", Shortcut: 'm'},
				{Action: keymap.ActionPanelListingFormatDialog, Label: "Listing format", Shortcut: 'f'},
				{Action: keymap.ActionPanelToggleCarousel, Label: "Carousel view", Shortcut: 'C'},
				{Action: keymap.ActionPanelToggleTree, Label: "Tree view", Shortcut: 'v'},
				{Action: keymap.ActionRemoteSFTPLink, Label: "SFTP ...", Shortcut: 'T'},
			},
		},
		{
			ID:         TopFile,
			PanelScope: PanelScopeNone,
			Label:      "File",
			Shortcut:   'f',
			Items: []Item{
				{Action: keymap.ActionFileView, Label: "View", Shortcut: 'V'},
				{Action: keymap.ActionFileEdit, Label: "Edit", Shortcut: 'e'},
				{Action: keymap.ActionCopy, Label: "Copy", Shortcut: 'c'},
				{Action: keymap.ActionFileExtract, Label: "Extract", Shortcut: 'x'},
				{Action: keymap.ActionFileChmod, Label: "Chmod", Shortcut: 'h'},
				{Action: keymap.ActionFileHardlink, Label: "Link", Shortcut: 'l'},
				{Action: keymap.ActionFileSymlink, Label: "Symlink", Shortcut: 's'},
				{Action: keymap.ActionFileChown, Label: "Chown", Shortcut: 'o'},
				{Action: keymap.ActionMenuFileChattr, Label: "Chattr", Shortcut: 't'},
				{Action: keymap.ActionMove, Label: "Rename/Move", Shortcut: 'r'},
				{Action: keymap.ActionFileFlatten, Label: "Flatten", Shortcut: 'F'},
				{Action: keymap.ActionFileMkdir, Label: "Mkdir", Shortcut: 'm'},
				{Action: keymap.ActionFileDelete, Label: "Delete", Shortcut: 'd'},
				{Separator: true},
				{Action: keymap.ActionPanelSelectGroup, Label: "Select group", Shortcut: 'g'},
				{Action: keymap.ActionPanelUnselectGroup, Label: "Unselect group", Shortcut: 'n'},
				{Action: keymap.ActionPanelInvertSelection, Label: "Invert selection", Shortcut: 'I'},
				{Separator: true},
				{Action: keymap.ActionAppQuit, Label: "Exit", Shortcut: 'i'},
			},
		},
		{
			ID:         TopCommand,
			PanelScope: PanelScopeNone,
			Label:      "Command",
			Shortcut:   'c',
			Items: []Item{
				{Action: keymap.ActionAppUserMenu, Label: "User menu", Shortcut: 'U'},
				{Action: keymap.ActionFileRunForEach, Label: "Run for each...", Shortcut: 'f'},
				{Action: keymap.ActionPanelToggleSplitOrientation, Label: "Toggle split orientation", Shortcut: 'O'},
				{Action: keymap.ActionPanelSwapPanes, Label: "Swap panels", Shortcut: 's'},
				{Action: keymap.ActionPanelComparePanels, Label: "Compare panels", Shortcut: 'p'},
				{Action: keymap.ActionPanelFindDuplicates, Label: "Find duplicates", Shortcut: 'd'},
				{Action: keymap.ActionBookmarkOpen, Label: "Bookmarks", Shortcut: 'b'},
				{Action: keymap.ActionBookmarkAdd, Label: "Add bookmark", Shortcut: 'a'},
				{Action: keymap.ActionPanelRefresh, Label: "Refresh", Shortcut: 'r'},
				{Action: keymap.ActionPanelExternalBrowser, Label: "External browser", Shortcut: 'e'},
			},
		},
		DisplayDefinition(),
		{
			ID:         TopOptions,
			PanelScope: PanelScopeNone,
			Label:      "Options",
			Shortcut:   'o',
			Items:      optionsItems,
		},
		{
			ID:         TopPanelRight,
			PanelScope: PanelScopeSecondary,
			Label:      "Right",
			Shortcut:   'r',
			Items: []Item{
				{Action: keymap.ActionFileQuickView, Label: "Quick view", Shortcut: 'q'},
				{Action: keymap.ActionPanelSortDialog, Label: "Sort...", Shortcut: 's'},
				{Action: keymap.ActionPanelToggleHidden, Label: "Toggle hidden", Shortcut: 'h'},
				{Action: keymap.ActionPanelRefresh, Label: "Refresh", Shortcut: 'r'},
				{Action: keymap.ActionPanelDiskUsageScan, Label: "Disk usage", Shortcut: 'u'},
				{Action: keymap.ActionPanelHistoryDialog, Label: "History...", Shortcut: 'y'},
				{Action: keymap.ActionPanelFindDialog, Label: "Find ...", Shortcut: 'i'},
				{Action: keymap.ActionPanelExternalBrowser, Label: "External browser", Shortcut: 'e'},
				{Action: keymap.ActionPanelMeta, Label: "Meta", Shortcut: 'm'},
				{Action: keymap.ActionPanelListingFormatDialog, Label: "Listing format", Shortcut: 'f'},
				{Action: keymap.ActionPanelToggleCarousel, Label: "Carousel view", Shortcut: 'C'},
				{Action: keymap.ActionPanelToggleTree, Label: "Tree view", Shortcut: 'v'},
				{Action: keymap.ActionRemoteSFTPLink, Label: "SFTP ...", Shortcut: 'T'},
			},
		},
	}
}

// DefaultIndex is the File menu, matching the most common v1 operations.
func DefaultIndex() int {
	return 1
}
