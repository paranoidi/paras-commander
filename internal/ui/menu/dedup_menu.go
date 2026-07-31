package menu

import "github.com/paranoidi/paras-commander/internal/keymap"

// DedupToggleEmptyMenuLabel returns the Actions menu label for dedup.toggle-empty:
// the text names the action that will run on the next activation.
func DedupToggleEmptyMenuLabel(ignoreEmpty bool) string {
	if ignoreEmpty {
		return "Show empty files"
	}
	return "Ignore empty files"
}

// DefinitionsDedup returns top menus while the find-duplicates view is active.
// ignoreEmpty is the current DedupViewState.IgnoreEmpty (empties hidden when true).
// treeDirs is true in directory-tree mode; sort is omitted when true.
func DefinitionsDedup(ignoreEmpty, treeDirs bool) []Definition {
	items := []Item{
		{Action: keymap.ActionDedupClose, Label: "Back to file view", Shortcut: 'b'},
		{Action: keymap.ActionPanelRefresh, Label: "Refresh", Shortcut: 'r'},
		{Action: keymap.ActionDedupCompare, Label: "Compare directories", Shortcut: 'd'},
	}
	if !treeDirs {
		items = append(items, Item{Action: keymap.ActionDedupToggleSort, Label: "Sort order", Shortcut: 's'})
	}
	items = append(items,
		Item{Action: keymap.ActionDedupToggleEmpty, Label: DedupToggleEmptyMenuLabel(ignoreEmpty), Shortcut: 'e'},
		Item{Action: keymap.ActionDedupToggleTree, Label: "Directory / group view", Shortcut: 'g'},
		Item{Action: keymap.ActionDedupCollapseAll, Label: "Collapse all", Shortcut: 'c'},
		Item{Action: keymap.ActionDedupExpandAll, Label: "Expand all", Shortcut: 'a'},
	)
	items = append(items,
		Item{Action: keymap.ActionPanelClearSelection, Label: "Unselect all", Shortcut: 'u'},
		Item{Action: keymap.ActionFileDelete, Label: "Delete marked", Shortcut: 'm'},
	)
	return []Definition{
		{
			ID:         TopDedup,
			PanelScope: PanelScopeNone,
			Label:      "Actions",
			Shortcut:   'a',
			Items:      items,
		},
		DisplayDefinition(),
		{
			ID:         TopFile,
			PanelScope: PanelScopeNone,
			Label:      "File",
			Shortcut:   'f',
			Items: []Item{
				{Action: keymap.ActionAppQuit, Label: "Exit", Shortcut: 'x'},
			},
		},
	}
}

// DedupDefinitions returns DefinitionsDedup() with KeyLabels resolved from global km plus optional dedup overlay.
func DedupDefinitions(global, dedup *keymap.Map, ignoreEmpty, treeDirs bool) []Definition {
	defs := DefinitionsDedup(ignoreEmpty, treeDirs)
	ApplyOverlayMenuKeyLabels(defs, global, dedup)
	return defs
}

// DefaultIndexDedup selects the Actions pulldown first.
func DefaultIndexDedup() int { return 0 }
