package menu

import "github.com/paranoidi/paras-commander/internal/keymap"

// DisplayDefinition is the shared Display pulldown (Commands / Messages / Jobs).
func DisplayDefinition() Definition {
	return Definition{
		ID:         TopDisplay,
		PanelScope: PanelScopeNone,
		Label:      "Display",
		Shortcut:   'd',
		Items: []Item{
			{Action: keymap.ActionCommandsOpen, Label: "Commands", Shortcut: 'c'},
			{Action: keymap.ActionMessagesOpen, Label: "Messages", Shortcut: 'm'},
			{Action: keymap.ActionJobsOpen, Label: "Jobs", Shortcut: 'j'},
		},
	}
}
