package menu

import "github.com/paranoidi/paras-commander/internal/keymap"

// CompareToggleEmptyMenuLabel returns the Actions menu label for compare.toggle-empty:
// the text names the action that will run on the next activation.
func CompareToggleEmptyMenuLabel(ignoreEmpty bool) string {
	if ignoreEmpty {
		return "Show empty files"
	}
	return "Ignore empty files"
}

// DefinitionsCompare returns top menus while the compare view is active.
func DefinitionsCompare(ignoreEmpty bool) []Definition {
	return []Definition{
		{
			ID:         TopCompare,
			PanelScope: PanelScopeNone,
			Label:      "Actions",
			Shortcut:   'a',
			Items: []Item{
				{Action: keymap.ActionCompareClose, Label: "Back to file view", Shortcut: 'b'},
				{Action: keymap.ActionCompareCycleFilter, Label: "Category", Shortcut: 'c'},
				{Action: keymap.ActionCompareRefresh, Label: "Refresh", Shortcut: 'r'},
				{Action: keymap.ActionCompareMerge, Label: "Merge", Shortcut: 'm'},
				{Action: keymap.ActionCompareToggleEmpty, Label: CompareToggleEmptyMenuLabel(ignoreEmpty), Shortcut: 'e'},
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

// CompareDefinitions returns DefinitionsCompare() with KeyLabels resolved from global km plus optional compare overlay.
func CompareDefinitions(global, compare *keymap.Map, ignoreEmpty bool) []Definition {
	defs := DefinitionsCompare(ignoreEmpty)
	ApplyOverlayMenuKeyLabels(defs, global, compare)
	return defs
}

// DefaultIndexCompare selects the Actions pulldown first.
func DefaultIndexCompare() int { return 0 }
