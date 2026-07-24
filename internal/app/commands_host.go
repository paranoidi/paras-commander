package app

import (
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// commandsHost implements apphandler/commands.Host for *App.
type commandsHost struct {
	appShellHost
}

func (h commandsHost) InactivePanel() *panel.State { return h.app.inactivePanel() }

func (h commandsHost) BrowserMenuDefinitions() []menu.Definition {
	return h.app.browserMenuDefinitions()
}

func (h commandsHost) SetTransientMessageBanner(log, banner string, urgency ui.MessageUrgency) {
	h.app.setTransientMessageBanner(log, banner, urgency)
}

func (h commandsHost) ClearTransientMessage() { h.app.clearTransientMessage() }

func (h commandsHost) CloseFileDialog() { h.app.closeFileDialog() }

func (h commandsHost) FocusedFileDialogField() *dialog.FileDialogField {
	return h.app.focusedField()
}

func (h commandsHost) RefreshAfterUserMenuCommand() { h.app.refreshAfterUserMenuCommand() }
