package app

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) openLeaderMenu(userMenu, copyMenu bool, items []ui.LeaderMenuItem) {
	a.model.LeaderMenu = ui.LeaderMenuState{Open: true, UserMenu: userMenu, CopyMenu: copyMenu, Items: items}
}

func (a *App) refreshLeaderMenuDirectKeys() {
	st := &a.model.LeaderMenu
	if st.UserMenu || st.CopyMenu || len(st.Items) == 0 {
		return
	}
	actionIdx := 0
	for i := range st.Items {
		if st.Items[i].GroupTitle != "" {
			continue
		}
		directKey := ""
		if a.config.UI.LeaderMenuShowDirectKeys && a.keys != nil && a.keys.Global != nil && actionIdx < len(a.leaderMenuActions) {
			directKey = a.keys.Global.MenuBindingLabel(a.leaderMenuActions[actionIdx])
		}
		st.Items[i].DirectKey = directKey
		actionIdx++
	}
}

func (a *App) toggleLeaderMenuDirectKeys() {
	a.config.UI.LeaderMenuShowDirectKeys = !a.config.UI.LeaderMenuShowDirectKeys
	a.refreshLeaderMenuDirectKeys()
	msg := "Function menu: chord hints on"
	urgency := ui.MessageUrgencyInfo
	if !a.config.UI.LeaderMenuShowDirectKeys {
		msg = "Function menu: chord hints off"
	}
	if err := a.persistPartial(map[string]interface{}{
		"ui": map[string]interface{}{
			"leader_menu_show_direct_keys": a.config.UI.LeaderMenuShowDirectKeys,
		},
	}); err != nil {
		msg = fmt.Sprintf("%s (config save failed: %v)", msg, err)
		urgency = ui.MessageUrgencyWarn
	}
	a.setTransientMessage(msg, urgency)
}

// openLeaderMenuStrip opens the bottom function menu when items fit; cancels an active quick filter first.
func (a *App) openLeaderMenuStrip(items []ui.LeaderMenuItem, userMenu, copyMenu bool, prefix string, onActivate func(int) bool) bool {
	if a.inQuickFilterUI() {
		a.cancelActiveQuickFilter()
	}
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	if len(ui.LeaderMenuVisibleItems(layout, items)) == 0 {
		a.setTransientMessage(prefix+": terminal too small", ui.MessageUrgencyWarn)
		return false
	}
	a.leaderMenuOnActivate = onActivate
	a.openLeaderMenu(userMenu, copyMenu, items)
	a.leaderMenuHiddenWarning(items, prefix)
	a.clearTransientMessage()
	return true
}

// openLeaderMenuDispatch wires action IDs parallel to non-group menu rows and opens the strip.
func (a *App) openLeaderMenuDispatch(items []ui.LeaderMenuItem, actions []string, userMenu, copyMenu bool, prefix string) {
	if len(actions) == 0 {
		a.setTransientMessage(prefix+": no entries configured", ui.MessageUrgencyWarn)
		return
	}
	a.leaderMenuActions = actions
	a.openLeaderMenuStrip(items, userMenu, copyMenu, prefix, func(i int) bool {
		if i < 0 || i >= len(actions) {
			return false
		}
		return a.dispatchActionLikeKeyboardShortcut(actions[i])
	})
}

func (a *App) builtinLeaderMenuOpen() bool {
	st := a.model.LeaderMenu
	return st.Open && !st.UserMenu && !st.CopyMenu
}

func (a *App) toggleBuiltinLeaderMenu() {
	if a.builtinLeaderMenuOpen() {
		a.closeLeaderMenu()
		return
	}
	a.openBuiltinLeaderMenu()
}

func (a *App) openBuiltinLeaderMenu() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	if a.keys == nil {
		return
	}
	entries := a.keys.LeaderMenuEntries()
	if len(entries) == 0 {
		a.setTransientMessage("Function menu: no entries configured", ui.MessageUrgencyWarn)
		return
	}
	var items []ui.LeaderMenuItem
	var actions []string
	for _, e := range entries {
		if e.GroupTitle != "" {
			items = append(items, ui.LeaderMenuItem{GroupTitle: e.GroupTitle, GroupColumn: e.GroupColumn})
			continue
		}
		directKey := ""
		if a.config.UI.LeaderMenuShowDirectKeys && a.keys.Global != nil {
			directKey = a.keys.Global.MenuBindingLabel(e.ActionID)
		}
		items = append(items, ui.LeaderMenuItem{
			Key:         e.Key,
			Label:       e.Label,
			GroupColumn: e.GroupColumn,
			DirectKey:   directKey,
		})
		actions = append(actions, e.ActionID)
	}
	a.openLeaderMenuDispatch(items, actions, false, false, "Function menu")
}

func (a *App) closeLeaderMenu() {
	a.model.LeaderMenu = ui.LeaderMenuState{}
	a.leaderMenuOnActivate = nil
	a.leaderMenuActions = nil
}

func (a *App) activateLeaderMenu(i int) bool {
	onActivate := a.leaderMenuOnActivate
	a.closeLeaderMenu()
	if onActivate != nil {
		return onActivate(i)
	}
	return false
}

func (a *App) handleLeaderMenuKey(event *tcell.EventKey) bool {
	if event.Key() == tcell.KeyF9 && a.model.LeaderMenu.UserMenu {
		a.editUserMenuConfigFromDialog()
		return false
	}
	st := &a.model.LeaderMenu
	if len(st.Items) == 0 {
		a.closeLeaderMenu()
		return false
	}

	switch event.Key() {
	case tcell.KeyEsc:
		a.closeLeaderMenu()
		return false
	case tcell.KeyF3:
		if !st.UserMenu && !st.CopyMenu {
			a.toggleLeaderMenuDirectKeys()
		}
		return false
	case tcell.KeyRune:
		if event.Modifiers()&(tcell.ModAlt|tcell.ModCtrl|tcell.ModMeta) == 0 {
			if i, ok := ui.LeaderMenuIndexForKey(st.Items, event.Rune()); ok {
				return a.activateLeaderMenu(i)
			}
		}
		return false
	default:
		return false
	}
}

func (a *App) leaderMenuHiddenWarning(items []ui.LeaderMenuItem, prefix string) {
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	if hidden := ui.LeaderMenuHiddenActionCount(layout, items); hidden > 0 {
		a.setTransientMessage(fmt.Sprintf("%s: %d entries hidden (terminal too small)", prefix, hidden), ui.MessageUrgencyWarn)
	}
}
