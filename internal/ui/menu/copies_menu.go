package menu

import "github.com/paranoidi/paras-commander/internal/keymap"

// DefinitionsCopies returns top menus while the Other copies view is active.
func DefinitionsCopies() []Definition {
	return []Definition{
		{
			ID:         TopCopies,
			PanelScope: PanelScopeNone,
			Label:      "Actions",
			Shortcut:   'a',
			Items: []Item{
				{Action: keymap.ActionCopiesClose, Label: "Back to file view", Shortcut: 'b'},
				{Action: keymap.ActionCopy, Label: "Copy to the other panel", Shortcut: 'c'},
				{Action: keymap.ActionMove, Label: "Move to the other panel", Shortcut: 'm'},
				{Action: keymap.ActionFileDelete, Label: "Delete", Shortcut: 'd'},
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

// CopiesDefinitions returns DefinitionsCopies with KeyLabels resolved from global km plus the
// view's overlay.
func CopiesDefinitions(global, copies *keymap.Map) []Definition {
	defs := DefinitionsCopies()
	ApplyOverlayMenuKeyLabels(defs, global, copies)
	return defs
}
