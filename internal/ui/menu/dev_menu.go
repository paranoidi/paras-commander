package menu

import "github.com/paranoidi/paras-commander/internal/keymap"

// DevDefinition returns the Dev pulldown (test helpers); omit from browser menus unless dev mode is on.
func DevDefinition() Definition {
	return Definition{
		ID:         TopDev,
		PanelScope: PanelScopeNone,
		Label:      "Dev",
		Shortcut:   'v',
		Items: []Item{
			{Action: keymap.ActionDevShowInfo, Label: "Show info", Shortcut: 's'},
			{Action: keymap.ActionDevShowWarn, Label: "Show warn", Shortcut: 'w'},
			{Action: keymap.ActionDevShowError, Label: "Show error", Shortcut: 'e'},
		},
	}
}
