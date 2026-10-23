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
			{Action: keymap.ActionDevShowSuccess, Label: "Show success", Shortcut: 'h'},
			{Action: keymap.ActionDevShowLongInfo, Label: "Long info", Shortcut: 'l'},
			{Action: keymap.ActionDevShowLongWarn, Label: "Long warn", Shortcut: 'o'},
			{Action: keymap.ActionDevShowLongError, Label: "Long error", Shortcut: 'n'},
			{Action: keymap.ActionDevShowLongSuccess, Label: "Long success", Shortcut: 'g'},
			{Action: keymap.ActionDevDemoDeleteBar, Label: "Delete bar demo", Shortcut: 'd'},
		},
	}
}
