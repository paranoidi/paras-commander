package app

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// cursorNameHintFlushPayload triggers a repaint after cursor-name-hint nav debounce clears.
type cursorNameHintFlushPayload struct{}

// clearCursorNameHintNavCoalesce stops the pending coalesce and re-enables the bottom name overlay.
func (a *App) clearCursorNameHintNavCoalesce() {
	a.cursorNameHintNav.Clear()
	a.cursorNameHintNavSkip.Store(false)
}

// armCursorNameHintNavCoalesceAfterListNav suppresses the bottom full-name overlay while the user
// is holding a navigation key; the overlay reappears once the debounce timer fires.
func (a *App) armCursorNameHintNavCoalesceAfterListNav() {
	if a.config.UI.KeyRepeatDebounceMS <= 0 {
		return
	}
	delay := time.Duration(a.config.UI.KeyRepeatDebounceMS) * time.Millisecond
	a.cursorNameHintNavSkip.Store(true)
	a.cursorNameHintNav.Reset(delay, func() {
		a.cursorNameHintNavSkip.Store(false)
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(cursorNameHintFlushPayload{}))
	})
}

// syncCursorNameHintNavCoalesceFlags propagates the atomic skip flag into panel state before painting.
func (a *App) syncCursorNameHintNavCoalesceFlags() {
	skip := a.cursorNameHintNavSkip.Load()
	a.model.Primary.CursorNameHintCoalesce = skip && a.model.ActivePanel == ui.PrimaryPanel
	a.model.Secondary.CursorNameHintCoalesce = skip && a.model.ActivePanel == ui.SecondaryPanel
}
