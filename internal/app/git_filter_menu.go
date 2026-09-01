package app

import (
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) toggleGitFilterMenu() {
	if a.gitFilterMenuOpen() {
		a.closeLeaderMenu()
		return
	}
	items := []ui.LeaderMenuItem{
		{Key: 'n', Label: "No filtering"},
		{Key: 's', Label: "Staged"},
		{Key: 'u', Label: "Unstaged"},
		{Key: 't', Label: "Tracked"},
		{Key: 'r', Label: "Untracked"},
	}
	filters := []*panel.EntryFilter{nil, panel.GitStagedFilter(), panel.GitUnstagedFilter(), panel.GitTrackedFilter(), panel.GitUntrackedFilter()}
	// onActivate's bool return is the app-quit signal (see handleLeaderMenuKey/InputModeLeaderMenu
	// in input.go), not a success flag — always return false here.
	a.openLeaderMenuStrip(items, false, false, false, true, "Git filter", func(i int) bool {
		if i < 0 || i >= len(filters) {
			return false
		}
		a.activePanel().SetEntryFilter(filters[i])
		return false
	})
}
