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
func DefinitionsDedup(ignoreEmpty bool) []Definition {
	return []Definition{
		{
			ID:         TopDedup,
			PanelScope: PanelScopeNone,
			Label:      "Actions",
			Shortcut:   'a',
			Items: []Item{
				{Action: keymap.ActionDedupClose, Label: "Back to file view", Shortcut: 'b'},
				{Action: keymap.ActionPanelRefresh, Label: "Refresh", Shortcut: 'r'},
				{Action: keymap.ActionDedupToggleSort, Label: "Sort order", Shortcut: 's'},
				{Action: keymap.ActionDedupToggleEmpty, Label: DedupToggleEmptyMenuLabel(ignoreEmpty), Shortcut: 'e'},
				{Action: keymap.ActionDedupToggleTree, Label: "Directory / group view", Shortcut: 'g'},
				{Action: keymap.ActionDedupCollapseAll, Label: "Collapse all", Shortcut: 'c'},
				{Action: keymap.ActionDedupExpandAll, Label: "Expand all", Shortcut: 'a'},
				{Action: keymap.ActionDedupMarkRedundant, Label: "Select to keep uniques in this folder", Shortcut: 'k'},
				{Action: keymap.ActionDedupMarkDuplicates, Label: "Select to delete duplicates in this folder", Shortcut: 'd'},
				{Action: keymap.ActionDedupMarkGroup, Label: "Toggle selection of whole group", Shortcut: 't'},
				{Action: keymap.ActionPanelClearSelection, Label: "Unmark all", Shortcut: 'u'},
				{Action: keymap.ActionFileDelete, Label: "Delete marked", Shortcut: 'm'},
			},
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
func DedupDefinitions(global, dedup *keymap.Map, ignoreEmpty bool) []Definition {
	defs := DefinitionsDedup(ignoreEmpty)
	ApplyOverlayMenuKeyLabels(defs, global, dedup)
	return defs
}

// DefaultIndexDedup selects the Actions pulldown first.
func DefaultIndexDedup() int { return 0 }
