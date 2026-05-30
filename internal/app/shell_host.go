package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// appShellHost implements shared apphandler host facets for *App.
type appShellHost struct{ app *App }

func (h appShellHost) LayoutForTerminalSize(w, height int) ui.Layout {
	return h.app.layoutForTerminalSize(w, height)
}

func (h appShellHost) SetTransientMessage(text string, urgency ui.MessageUrgency) {
	h.app.setTransientMessage(text, urgency)
}

func (h appShellHost) SetErrorMessage(title string, err error) {
	h.app.setErrorMessage(title, err)
}

func (h appShellHost) ActivePanel() *panel.State { return h.app.activePanel() }

func (h appShellHost) HandleQuit() bool { return h.app.handleQuit() }

func (h appShellHost) HandleQuitImmediate() bool { return h.app.handleQuitImmediate() }

func (h appShellHost) OpenMenu() { h.app.openMenu() }

func (h appShellHost) OpenMenuByShortcut(shortcut rune) bool {
	return h.app.openMenuByShortcut(shortcut)
}

func (h appShellHost) Dispatch(actionID string) { h.app.dispatch(actionID) }

func (h appShellHost) TryDispatchAuxiliaryScreens(actionID string) bool {
	return h.app.tryDispatchAuxiliaryScreens(actionID)
}

func (h appShellHost) ActionFromKeyEvent(ev *tcell.EventKey) string {
	return h.app.actionFromKeyEvent(ev)
}
