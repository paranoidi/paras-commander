package app

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
)

// armNavParentBackspaceGuard activates the guard and (re)starts its release timer.
// Call when backspace erases the last filter character, and again on every
// subsequent guarded backspace event while the key is still held down.
// The guard is cleared when the timer fires (no more key-repeats → key released),
// allowing the next deliberate backspace to navigate to the parent directory.
func (a *App) armNavParentBackspaceGuard() {
	if a.config.UI.KeyRepeatDebounceMS <= 0 {
		return
	}
	a.navParentBackspaceGuarded.Store(true)
	delay := time.Duration(a.config.UI.KeyRepeatDebounceMS) * time.Millisecond
	a.navParentBackspaceDebounce.Reset(delay, func() {
		a.navParentBackspaceGuarded.Store(false)
	})
}

// isNavParentBackspaceEvent reports whether event is an unmodified backspace
// that resolves to nav.parent.
func isNavParentBackspaceEvent(event *tcell.EventKey, action string) bool {
	if action != keymap.ActionNavParent {
		return false
	}
	return (event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2) &&
		event.Modifiers() == tcell.ModNone
}
