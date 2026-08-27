package app

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/app/helpkeys"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) openLeaderMenu(userMenu, copyMenu, previewMenu bool, items []ui.LeaderMenuItem) {
	a.model.LeaderMenu = ui.LeaderMenuState{Open: true, UserMenu: userMenu, CopyMenu: copyMenu, PreviewMenu: previewMenu, Items: items}
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
		if a.config.UI.LeaderMenuShowDirectKeys && a.keys != nil && actionIdx < len(a.leaderMenuActions) {
			keyMap := a.keys.Global
			if st.PreviewMenu {
				keyMap = a.keys.FilePreview
			}
			if keyMap != nil {
				directKey = keyMap.MenuBindingLabel(a.leaderMenuActions[actionIdx])
			}
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
func (a *App) openLeaderMenuStrip(items []ui.LeaderMenuItem, userMenu, copyMenu, previewMenu bool, prefix string, onActivate func(int) bool) bool {
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
	a.openLeaderMenu(userMenu, copyMenu, previewMenu, items)
	a.leaderMenuHiddenWarning(items, prefix)
	a.clearTransientMessage()
	return true
}

// openLeaderMenuDispatch wires action IDs parallel to non-group menu rows and opens the strip.
// activate runs the chosen action; the browser's builtin/copy menus pass
// dispatchActionLikeKeyboardShortcut, while per-view menus (openViewLeaderMenu) pass
// activateHelpAction so the action runs through that view's own controller.
func (a *App) openLeaderMenuDispatch(items []ui.LeaderMenuItem, actions []string, userMenu, copyMenu bool, prefix string, activate func(actionID string) bool) {
	if len(actions) == 0 {
		a.setTransientMessage(prefix+": no entries configured", ui.MessageUrgencyWarn)
		return
	}
	a.leaderMenuActions = actions
	a.openLeaderMenuStrip(items, userMenu, copyMenu, false, prefix, func(i int) bool {
		if i < 0 || i >= len(actions) {
			return false
		}
		return activate(actions[i])
	})
}

func (a *App) builtinLeaderMenuOpen() bool {
	st := a.model.LeaderMenu
	return st.Open && !st.UserMenu && !st.CopyMenu && !st.PreviewMenu
}

func (a *App) previewLeaderMenuOpen() bool {
	st := a.model.LeaderMenu
	return st.Open && st.PreviewMenu
}

func (a *App) toggleBuiltinLeaderMenu() {
	if a.builtinLeaderMenuOpen() {
		a.closeLeaderMenu()
		return
	}
	if a.model.ViewMode == ui.ViewBrowser {
		a.openBuiltinLeaderMenu()
		return
	}
	a.openViewLeaderMenu()
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
	items, actions := a.buildLeaderMenuItems(entries)
	a.openLeaderMenuDispatch(items, actions, false, false, "Function menu", a.dispatchActionLikeKeyboardShortcut)
}

// openViewLeaderMenu opens the `:` leader menu scoped to the current auxiliary view's own
// actions (Compare, Dedup, Jobs, Commands, Messages), built from
// keymap.Bundle.LeaderMenuEntriesForView. Activation reuses activateHelpAction — the same
// per-view dispatch the F1 help dialog uses — so entries run through that view's own
// controller instead of the browser-only dispatchActionLikeKeyboardShortcut.
func (a *App) openViewLeaderMenu() {
	if a.keys == nil {
		return
	}
	hc := a.helpContextFor(a.model.ViewMode)
	prefix := hc.menuTitle
	if prefix == "" {
		return
	}
	vm := helpkeys.ViewMask(a.model.ViewMode)
	entries := a.keys.LeaderMenuEntriesForView(vm)
	if len(entries) == 0 {
		a.setTransientMessage(prefix+": no entries configured", ui.MessageUrgencyWarn)
		return
	}
	items, actions := a.buildLeaderMenuItems(entries)
	a.openLeaderMenuDispatch(items, actions, false, false, prefix, a.activateHelpAction)
}

// buildLeaderMenuItems converts leader-menu entries into UI items plus their parallel action-ID
// slice, resolving each non-group entry's direct-key hint from the global keymap. Shared by
// openBuiltinLeaderMenu and openViewLeaderMenu, which differ only in their entries source.
func (a *App) buildLeaderMenuItems(entries []keymap.LeaderMenuEntry) ([]ui.LeaderMenuItem, []string) {
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
	return items, actions
}

// dispatchLeaderLetterDirectFire fires the current auxiliary view's own leader-menu action bound
// to event's rune directly, without opening the `:` menu, when vi-motion mode is on — the same
// mechanism the browser already uses (see input.go's InputModeNormal handling), scoped per view
// via keymap.Bundle.ActionForLeaderKeyInView so cross-view letter reuse (e.g. Compare's `c` =
// Close vs. Dedup's `c` = Collapse) stays unambiguous. Returns false (no-op) when vi-motion mode
// is off, event isn't a plain letter, or no action in this view's leader menu is bound to it.
func (a *App) dispatchLeaderLetterDirectFire(event *tcell.EventKey) bool {
	if !a.model.ViMotionMode || !keymap.IsPlainPrintableRune(event) {
		return false
	}
	actionID, ok := a.keys.ActionForLeaderKeyInView(event.Rune(), helpkeys.ViewMask(a.model.ViewMode))
	if !ok {
		return false
	}
	a.activateHelpAction(actionID)
	return true
}

// dispatchAuxiliaryViewCommonKeys handles the key/action set shared by every auxiliary view's key
// handler (Compare, Dedup, Messages): app quit, quit-immediate, F9 menu, leader-menu toggle,
// vi-motion leader-letter direct-fire, and F-key menu shortcuts. Returns handled=true when event
// was fully handled here, in which case the caller should return result immediately; otherwise the
// caller continues with its own view-specific dispatch.
func (a *App) dispatchAuxiliaryViewCommonKeys(event *tcell.EventKey, nextAction string) (result, handled bool) {
	if nextAction == keymap.ActionAppQuit {
		return a.handleQuit(), true
	}
	if nextAction == keymap.ActionAppQuitImmediate {
		return a.handleQuitImmediate(), true
	}
	if nextAction == keymap.ActionAppOpenMenu {
		a.openMenu()
		return false, true
	}
	if nextAction == keymap.ActionAppLeaderMenu {
		a.toggleBuiltinLeaderMenu()
		return false, true
	}
	if a.dispatchLeaderLetterDirectFire(event) {
		return false, true
	}
	if a.tryOpenMenuByShortcut(event) {
		return false, true
	}
	return false, false
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
		if st.UserMenu && len(a.userMenuStack) > 0 {
			parent := a.userMenuStack[len(a.userMenuStack)-1]
			a.userMenuStack = a.userMenuStack[:len(a.userMenuStack)-1]
			a.openUserMenuLevel(parent)
			return false
		}
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
