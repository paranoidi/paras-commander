package menu

import "github.com/paranoidi/paras-commander/internal/keymap"

// DefinitionsDedup returns top menus while the find-duplicates view is active.
func DefinitionsDedup() []Definition {
	return []Definition{
		{
			ID:         TopDedup,
			PanelScope: PanelScopeNone,
			Label:      "Actions",
			Shortcut:   'a',
			Items: []Item{
				{Action: keymap.ActionDedupClose, Label: "Back to file view", Shortcut: 'b'},
				{Action: keymap.ActionDedupRefresh, Label: "Refresh", Shortcut: 'r'},
				{Action: keymap.ActionDedupToggleSort, Label: "Sort order", Shortcut: 's'},
				{Action: keymap.ActionDedupToggleEmpty, Label: "Ignore empty files", Shortcut: 'e'},
				{Action: keymap.ActionDedupMarkRedundant, Label: "Keep uniques in this folder", Shortcut: 'k'},
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
func DedupDefinitions(global, dedup *keymap.Map) []Definition {
	defs := DefinitionsDedup()
	ApplyOverlayMenuKeyLabels(defs, global, dedup)
	return defs
}

// DefaultIndexDedup selects the Actions pulldown first.
func DefaultIndexDedup() int { return 0 }
