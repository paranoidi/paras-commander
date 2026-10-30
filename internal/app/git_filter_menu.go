package app

import (
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) toggleGitFilterMenu() {
	if a.leaderMenuOpen(ui.LeaderMenuGitFilter) {
		a.closeLeaderMenu()
		return
	}
	// Grouped strip: two columns, which the leader-menu layout centers on the middle slots.
	items := []ui.LeaderMenuItem{
		{GroupTitle: "Filter", GroupColumn: 0},
		{Key: 'n', Label: "No filtering", GroupColumn: 0},
		{Key: 's', Label: "Staged", GroupColumn: 0},
		{Key: 'u', Label: "Unstaged", GroupColumn: 0},
		{Key: 't', Label: "Tracked", GroupColumn: 0},
		{Key: 'r', Label: "Untracked", GroupColumn: 0},
		{GroupTitle: "Repository", GroupColumn: 1},
		{Key: 'c', Label: "Commit", GroupColumn: 1},
		{Key: 'p', Label: "Pull", GroupColumn: 1},
		{Key: 'P', Label: "Push", GroupColumn: 1},
	}
	filters := []*panel.EntryFilter{nil, panel.GitStagedFilter(), panel.GitUnstagedFilter(), panel.GitTrackedFilter(), panel.GitUntrackedFilter()}
	// onActivate's bool return is the app-quit signal (see handleLeaderMenuKey/InputModeLeaderMenu
	// in input.go), not a success flag — always return false here.
	a.openLeaderMenuStrip(items, ui.LeaderMenuGitFilter, "Git", func(i int) bool {
		switch i - len(filters) {
		case 0:
			a.gitCommitActive()
			return false
		case 1:
			a.gitRemoteActive("pull")
			return false
		case 2:
			a.gitRemoteActive("push")
			return false
		}
		if i < 0 || i >= len(filters) {
			return false
		}
		a.activePanel().SetEntryFilter(filters[i])
		return false
	})
}
