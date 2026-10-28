package menu

import "github.com/paranoidi/paras-commander/internal/keymap"

// DisplayDefinition is the shared Display pulldown (Commands / Messages / Jobs / Open shell /
// Status commands).
func DisplayDefinition() Definition {
	return Definition{
		ID:         TopDisplay,
		PanelScope: PanelScopeNone,
		Label:      "Display",
		Shortcut:   'd',
		// Alt+D belongs to disk-usage chords.
		MenuBarOnly: true,
		Items: []Item{
			{Action: keymap.ActionCommandsOpen, Label: "Commands", Shortcut: 'c'},
			{Action: keymap.ActionMessagesOpen, Label: "Messages", Shortcut: 'm'},
			{Action: keymap.ActionDedupOpen, Label: "Duplicates", Shortcut: 'd'},
			{Action: keymap.ActionJobsOpen, Label: "Jobs", Shortcut: 'j'},
			{Action: keymap.ActionAppDropToShell, Label: "Open shell", Shortcut: 'o'},
			{Action: keymap.ActionUIStatusCommands, Label: "Status commands", Shortcut: 's'},
		},
	}
}
