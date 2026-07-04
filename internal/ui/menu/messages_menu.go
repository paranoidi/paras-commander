package menu

import "github.com/paranoidi/paras-commander/internal/keymap"

// DefinitionsMessages returns top menus while the Messages view is active.
func DefinitionsMessages() []Definition {
	return []Definition{
		{
			ID:         TopMessages,
			PanelScope: PanelScopeNone,
			Label:      "Actions",
			Shortcut:   'a',
			Items: []Item{
				{Action: keymap.ActionMessagesClear, Label: "Clear messages", Shortcut: 'c'},
				{Action: keymap.ActionMessagesClose, Label: "Back to file view", Shortcut: 'b'},
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

// MessagesDefinitions returns DefinitionsMessages() with KeyLabels resolved from global km plus optional Messages overlay.
func MessagesDefinitions(global, messages *keymap.Map) []Definition {
	defs := DefinitionsMessages()
	ApplyOverlayMenuKeyLabels(defs, global, messages)
	return defs
}

// DefaultIndexMessages selects the Messages pulldown first.
func DefaultIndexMessages() int { return 0 }
