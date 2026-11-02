package app

import (
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// gitMenuActions are the Actions group's rows, each dispatched as its keyboard action. The
// letters are the uppercase twins of the matching filter letters (s/u/r).
var gitMenuActions = []struct {
	key    rune
	label  string
	action string
}{
	{'S', "Stage", keymap.ActionGitStage},
	{'U', "Unstage", keymap.ActionGitUnstage},
	{'R', "Untrack", keymap.ActionGitUntrack},
}

func (a *App) toggleGitFilterMenu() {
	if a.leaderMenuOpen(ui.LeaderMenuGitFilter) {
		a.closeLeaderMenu()
		return
	}
	// Grouped strip: three columns, which the leader-menu layout centers on the slots.
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
		{GroupTitle: "Actions", GroupColumn: 2},
	}
	filters := []*panel.EntryFilter{nil, panel.GitStagedFilter(), panel.GitUnstagedFilter(), panel.GitTrackedFilter(), panel.GitUntrackedFilter()}
	const repoRows = 3
	// leaderMenuActions runs parallel to the non-group rows so direct-key hints (F3) resolve;
	// filter and repository rows have no action.
	actions := make([]string, len(filters)+repoRows, len(filters)+repoRows+len(gitMenuActions))
	for _, act := range gitMenuActions {
		items = append(items, ui.LeaderMenuItem{Key: act.key, Label: act.label, GroupColumn: 2})
		actions = append(actions, act.action)
	}
	a.leaderMenuActions = actions
	// onActivate's bool return is the app-quit signal (see handleLeaderMenuKey/InputModeLeaderMenu
	// in input.go), not a success flag — always return false here.
	opened := a.openLeaderMenuStrip(items, ui.LeaderMenuGitFilter, "Git", func(i int) bool {
		if i >= 0 && i < len(actions) && actions[i] != "" {
			return a.dispatchActionLikeKeyboardShortcut(actions[i])
		}
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
	if opened {
		a.refreshLeaderMenuDirectKeys()
	}
}
