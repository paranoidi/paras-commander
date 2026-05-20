package menu

import "github.com/paranoidi/paras-commander/internal/keymap"

// DefinitionsJobs returns top menus while the jobs view is active (KeyLabels filled via JobsDefinitions).
func DefinitionsJobs() []Definition {
	return []Definition{
		{
			ID:         TopJobs,
			PanelScope: PanelScopeNone,
			Label:      "Jobs",
			Items: []Item{
				{Action: keymap.ActionJobsCancel, Label: "Cancel job", Shortcut: 'c'},
				{Action: keymap.ActionJobsPause, Label: "Pause queued job", Shortcut: 'p'},
				{Action: keymap.ActionJobsResume, Label: "Resume paused job", Shortcut: 'r'},
				{Action: keymap.ActionJobsQueueUp, Label: "Move up in queue", Shortcut: 'u'},
				{Action: keymap.ActionJobsQueueDown, Label: "Move down in queue", Shortcut: 'd'},
				{Action: keymap.ActionJobsClearFinished, Label: "Clear finished", Shortcut: 'k'},
				{Action: keymap.ActionJobsClose, Label: "Back to file view", Shortcut: 'b'},
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

// JobsDefinitions returns DefinitionsJobs() with KeyLabels resolved from global km plus optional jobs overlay.
func JobsDefinitions(global, jobs *keymap.Map) []Definition {
	defs := DefinitionsJobs()
	ApplyJobsMenuKeyLabels(defs, global, jobs)
	return defs
}

// DefaultIndexJobs selects the Jobs pulldown first.
func DefaultIndexJobs() int {
	return 0
}
