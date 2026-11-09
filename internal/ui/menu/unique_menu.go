package menu

import "github.com/paranoidi/paras-commander/internal/keymap"

// DefinitionsUnique returns top menus while the Find unique files view is active.
func DefinitionsUnique() []Definition {
	return []Definition{
		{
			ID:         TopUnique,
			PanelScope: PanelScopeNone,
			Label:      "Actions",
			Shortcut:   'a',
			Items: []Item{
				{Action: keymap.ActionUniqueClose, Label: "Back to file view", Shortcut: 'b'},
				{Action: keymap.ActionPanelRefresh, Label: "Refresh", Shortcut: 'r'},
				{Action: keymap.ActionCopy, Label: "Copy to the other panel", Shortcut: 'c'},
				{Action: keymap.ActionMove, Label: "Move to the other panel", Shortcut: 'm'},
				{Action: keymap.ActionFileDelete, Label: "Delete", Shortcut: 'd'},
				{Action: keymap.ActionUniqueHashUnverified, Label: "Hash unverified files", Shortcut: 'h'},
				{Action: keymap.ActionPanelToggleHidden, Label: "Show / hide hidden files", Shortcut: 'i'},
				{Action: keymap.ActionUniqueCollapseAll, Label: "Collapse all", Shortcut: 'l'},
				{Action: keymap.ActionUniqueExpandAll, Label: "Expand all", Shortcut: 'e'},
				{Action: keymap.ActionPanelSelectRange, Label: "Select range", Shortcut: 'k'},
				{Action: keymap.ActionPanelClearSelection, Label: "Unselect all", Shortcut: 'u'},
			},
		},
		DisplayDefinition(),
		{
			ID:         TopFile,
			PanelScope: PanelScopeNone,
			Label:      "File",
			Shortcut:   'f',
			Items: []Item{
				{Action: keymap.ActionAppQuit, Label: "Exit", Shortcut: 'q'},
			},
		},
	}
}

// UniqueDefinitions returns DefinitionsUnique with KeyLabels resolved from global km plus the
// view's overlay.
func UniqueDefinitions(global, unique *keymap.Map) []Definition {
	defs := DefinitionsUnique()
	ApplyOverlayMenuKeyLabels(defs, global, unique)
	return defs
}
