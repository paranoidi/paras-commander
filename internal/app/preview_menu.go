package app

import "github.com/paranoidi/paras-commander/internal/ui"

func (a *App) togglePreviewLeaderMenu() {
	if a.previewLeaderMenuOpen() {
		a.closeLeaderMenu()
		return
	}
	a.openPreviewLeaderMenu()
}

// openPreviewLeaderMenu opens the `:` leader menu scoped to the fullscreen file preview (F3)
// view. Activation dispatches through previewCtrl.TryFilePreviewMenuAction, not the generic
// action dispatcher, so diff-hunk navigation and the fullscreen-only actions (reload,
// search-start) behave exactly like their direct keys inside the preview.
func (a *App) openPreviewLeaderMenu() {
	if a.model.ViewMode != ui.ViewFilePreview {
		return
	}
	if a.keys == nil {
		return
	}
	entries := a.keys.PreviewMenuEntries()
	if len(entries) == 0 {
		a.setTransientMessage("Preview menu: no entries configured", ui.MessageUrgencyWarn)
		return
	}
	var items []ui.LeaderMenuItem
	var actions []string
	for _, e := range entries {
		directKey := ""
		if a.config.UI.LeaderMenuShowDirectKeys && a.keys.FilePreview != nil {
			directKey = a.keys.FilePreview.MenuBindingLabel(e.ActionID)
		}
		items = append(items, ui.LeaderMenuItem{Key: e.Key, Label: e.Label, DirectKey: directKey})
		actions = append(actions, e.ActionID)
	}
	a.leaderMenuActions = actions
	a.openLeaderMenuStrip(items, false, false, true, false, "Preview menu", func(i int) bool {
		if i < 0 || i >= len(actions) {
			return false
		}
		return a.previewCtrl.TryFilePreviewMenuAction(actions[i])
	})
}
