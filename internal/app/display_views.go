package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
)

// tryDispatchAuxiliaryScreens switches between Jobs, Commands, and Messages screens.
// Returns true when actionID is one of the display-screen open actions (consumed).
func (a *App) tryDispatchAuxiliaryScreens(actionID string) bool {
	switch actionID {
	case keymap.ActionJobsOpen, keymap.ActionCommandsOpen, keymap.ActionMessagesOpen:
		a.dispatch(actionID)
		return true
	default:
		return false
	}
}

// tryOpenMenuByShortcut opens a top menu pulldown when shortcut matches the active menu bar.
func (a *App) tryOpenMenuByShortcut(event *tcell.EventKey) bool {
	if event.Key() != tcell.KeyRune || !keymap.AltLetterModifiers(event.Modifiers()) {
		return false
	}
	return a.openMenuByShortcut(event.Rune())
}
