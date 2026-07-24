package app

import (
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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
	if !a.model.QuickAction.Open {
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
		a.reloadUserMenuDialog()
	}
}

func (a *App) reloadUserMenuDialog() {
	st := &a.model.QuickAction
	if !st.Open {
		return
	}
	prevTitle := ""
	prevKey := ""
	if st.Selected >= 0 && st.Selected < len(a.userMenuVisible) {
		prevTitle = a.userMenuVisible[st.Selected].Title
		prevKey = a.userMenuVisible[st.Selected].Key
	}

	menuPath := a.userMenuPath
	if menuPath == "" {
		var err error
		menuPath, err = a.resolveUserMenuEditPath()
		if err != nil {
			a.setErrorMessage("User menu", err)
			a.closeQuickAction()
			return
		}
	}

	mf, err := usermenu.LoadFile(menuPath)
	if err != nil {
		a.setUserMenuCritical(err)
		a.closeQuickAction()
		return
	}
	if err := mf.ValidatePoolRefs(usermenu.PoolNameSet(a.workPools.Names())); err != nil {
		a.setUserMenuCritical(err)
		a.closeQuickAction()
		return
	}
	if len(mf.Entries) == 0 {
		a.setTransientMessage("User menu: no entries (edit with Shift+F2)", ui.MessageUrgencyWarn)
		a.closeQuickAction()
		return
	}

	active := a.panelByID(a.model.ActivePanel)
	other := a.panelByID(a.inactivePanelID())
	ctx := &usermenu.EvalContext{Active: active, Other: other}
	visible, defIdx, err := usermenu.FilterVisible(mf, ctx)
	if err != nil {
		a.setUserMenuCritical(err)
		a.closeQuickAction()
		return
	}
	if len(visible) == 0 {
		a.setTransientMessage("User menu: no visible entries", ui.MessageUrgencyWarn)
		a.closeQuickAction()
		return
	}

	selected := userMenuEntryIndexByKeyOrTitle(visible, prevKey, prevTitle, defIdx)

	a.userMenuVisible = visible
	a.userMenuPath = menuPath
	st.Items = userMenuQuickActionItems(visible)
	st.Selected = selected

	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	vr := dialog.QuickActionViewportRows(layout, len(visible))
	dialog.QuickActionEnsureScroll(st, vr)
}

func userMenuEntryIndexByKeyOrTitle(entries []usermenu.MenuEntry, key, title string, fallback int) int {
	if key != "" {
		for i, e := range entries {
			if e.Key == key {
				return i
			}
		}
	}
	if title != "" {
		for i, e := range entries {
			if e.Title == title {
				return i
			}
		}
	}
	if fallback >= 0 && fallback < len(entries) {
		return fallback
	}
	return 0
}
