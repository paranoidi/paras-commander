package app

import (
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/usermenu"
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
	if a.openUserMenuEditor(path) {
		a.reloadLeaderMenu()
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

	mf, err := usermenu.LoadFile(menuPath)
	if err != nil {
		a.setUserMenuCritical(err)
		a.closeLeaderMenu()
		return
	}
	if err := mf.ValidatePoolRefs(usermenu.PoolNameSet(a.workPools.Names())); err != nil {
		a.setUserMenuCritical(err)
		a.closeLeaderMenu()
		return
	}
	if len(mf.Entries) == 0 {
		a.setTransientMessage("User menu: no entries (edit with Shift+F2)", ui.MessageUrgencyWarn)
		a.closeLeaderMenu()
		return
	}

	active := a.panelByID(a.model.ActivePanel)
	other := a.panelByID(a.inactivePanelID())
	ctx := &usermenu.EvalContext{Active: active, Other: other}
	visible, _, err := usermenu.FilterVisible(mf, ctx)
	if err != nil {
		a.setUserMenuCritical(err)
		a.closeLeaderMenu()
		return
	}
	if len(visible) == 0 {
		a.setTransientMessage("User menu: no visible entries", ui.MessageUrgencyWarn)
		a.closeLeaderMenu()
		return
	}

	a.userMenuVisible = visible
	a.userMenuPath = menuPath
	a.model.LeaderMenu.Items = userMenuLeaderMenuItems(visible)
	a.leaderMenuHiddenWarning(a.model.LeaderMenu.Items, "User menu")
}
