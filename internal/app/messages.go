package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func (a *App) toggleMessagesView() {
	if a.model.ViewMode == ui.ViewMessages {
		a.closeMessagesView()
		return
	}
	a.openMessagesView()
}

func (a *App) openMessagesView() {
	a.model.ViewMode = ui.ViewMessages
	a.model.ActiveSubFocus = ui.SubFocusFileList
	a.model.MenuDefinitions = menu.MessagesDefinitions(a.keys, a.keysMessages)
	a.model.Menu.ActiveMenu = menu.DefaultIndexMessages()
	if n := len(a.model.MessageLog); n > 0 {
		a.model.MessagesView.Selected = n - 1
	} else {
		a.model.MessagesView.Selected = 0
	}
	a.model.MessagesView.ListScroll = 0
	a.ensureMessagesViewSelectionVisible()
}

func (a *App) closeMessagesView() {
	a.model.ViewMode = ui.ViewBrowser
	a.model.ActiveSubFocus = ui.SubFocusFileList
	a.model.MenuDefinitions = a.browserMenuDefinitions()
	a.model.Menu.ActiveMenu = menu.DefaultIndex()
	a.model.MessagesView = ui.MessagesViewState{}
}

func (a *App) ensureMessagesViewSelectionVisible() {
	n := len(a.model.MessageLog)
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		a.model.MessagesView.EnsureSelectionVisible(n, 0)
		return
	}
	rect := ui.MergeTwinPanelRects(layout.Left, layout.Right)
	visible := ui.PanelListRows(rect)
	a.model.MessagesView.EnsureSelectionVisible(n, visible)
}

func (a *App) tryDispatchMessages(actionID string) bool {
	switch actionID {
	case keymap.ActionMessagesOpen:
		a.toggleMessagesView()
		return true
	case keymap.ActionMessagesClose:
		if a.model.ViewMode == ui.ViewMessages {
			a.closeMessagesView()
		}
		return true
	default:
		return false
	}
}

func (a *App) handleMessagesViewKey(event *tcell.EventKey) bool {
	switch event.Key() {
	case tcell.KeyEsc:
		a.closeMessagesView()
		return false
	}

	nextAction := a.actionFromKeyEvent(event)
	if nextAction == keymap.ActionAppQuit {
		return a.handleQuit()
	}
	if nextAction == keymap.ActionAppQuitImmediate {
		return a.handleQuitImmediate()
	}
	if nextAction == keymap.ActionAppOpenMenu {
		a.openMenu()
		return false
	}
	if nextAction != "" && a.tryDispatchMessages(nextAction) {
		return false
	}
	if nextAction == keymap.ActionPanelExternalBrowser {
		a.dispatch(nextAction)
		return false
	}

	n := len(a.model.MessageLog)
	switch event.Key() {
	case tcell.KeyUp:
		if a.model.MessagesView.Selected > 0 {
			a.model.MessagesView.Selected--
		}
		a.ensureMessagesViewSelectionVisible()
	case tcell.KeyDown:
		if n > 0 && a.model.MessagesView.Selected < n-1 {
			a.model.MessagesView.Selected++
		}
		a.ensureMessagesViewSelectionVisible()
	case tcell.KeyPgUp:
		a.model.MessagesView.Selected = max(0, a.model.MessagesView.Selected-5)
		a.ensureMessagesViewSelectionVisible()
	case tcell.KeyPgDn:
		if n > 0 {
			a.model.MessagesView.Selected = min(n-1, a.model.MessagesView.Selected+5)
		}
		a.ensureMessagesViewSelectionVisible()
	case tcell.KeyHome:
		a.model.MessagesView.Selected = 0
		a.ensureMessagesViewSelectionVisible()
	case tcell.KeyEnd:
		if n > 0 {
			a.model.MessagesView.Selected = n - 1
		}
		a.ensureMessagesViewSelectionVisible()
	}
	return false
}
