package app

import (
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) resolveUserMenuEditPath() (string, error) {
	menuPath, warns := a.resolveUserMenuContext()
	for _, w := range warns {
		a.setTransientMessage(w, ui.MessageUrgencyWarn)
	}
	if menuPath == "" {
		return a.ensureGlobalUserMenuStub()
	}
	return menuPath, nil
}

func (a *App) editUserMenuConfigFromDialog() {
	if !a.model.LeaderMenu.Open || !a.model.LeaderMenu.UserMenu {
		return
	}
	path := a.userMenuPath
	if path == "" {
		var err error
		path, err = a.resolveUserMenuEditPath()
		if err != nil {
			a.setErrorMessage("User menu", err)
			return
		}
	}
	if !a.openUserMenuEditor(path) {
		return
	}
	// openUserMenuEditor already set an "updated documentation"/"edited" confirmation
	// message; reloadLeaderMenu redraws the strip via openUserMenuLevel, which clears it
	// (see openLeaderMenuStrip). Restore it unless the reload replaced it with something
	// more urgent (a fresh warning, or an error that also closes the strip).
	editMsg, editUrg := a.model.Message, a.model.MessageUrgency
	a.reloadLeaderMenu()
	if a.model.LeaderMenu.Open && a.model.Message == "" {
		a.setTransientMessage(editMsg, editUrg)
	}
}

func (a *App) reloadLeaderMenu() {
	if !a.model.LeaderMenu.Open || !a.model.LeaderMenu.UserMenu {
		return
	}

	menuPath := a.userMenuPath
	if menuPath == "" {
		var err error
		menuPath, err = a.resolveUserMenuEditPath()
		if err != nil {
			a.setErrorMessage("User menu", err)
			a.closeLeaderMenu()
			return
		}
	}

	visible, warnings, ok := a.loadUserMenuVisible(menuPath)
	if !ok {
		a.closeLeaderMenu()
		return
	}

	a.userMenuPath = menuPath
	a.userMenuWarnings = warnings
	a.userMenuStack = nil
	a.openUserMenuLevel(visible)
}
