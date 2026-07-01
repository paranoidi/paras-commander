package menu

import "github.com/paranoidi/paras-commander/internal/keymap"

// DefinitionsCommands returns top menus while the Commands view is active.
func DefinitionsCommands() []Definition {
	return []Definition{
		{
			ID:         TopCommands,
			PanelScope: PanelScopeNone,
			Label:      "Actions",
			Shortcut:   'a',
			Items: []Item{
				{Action: keymap.ActionCommandsTerminate, Label: "Terminate command", Shortcut: 't'},
				{Action: keymap.ActionCommandsKill, Label: "Kill command", Shortcut: 'k'},
				{Action: keymap.ActionCommandsClose, Label: "Back to file view", Shortcut: 'b'},
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

// CommandsDefinitions returns DefinitionsCommands() with KeyLabels resolved from global km plus optional Commands overlay.
func CommandsDefinitions(global, commands *keymap.Map) []Definition {
	defs := DefinitionsCommands()
	ApplyCommandsMenuKeyLabels(defs, global, commands)
	return defs
}

// DefaultIndexCommands selects the Commands pulldown first.
func DefaultIndexCommands() int { return 0 }
