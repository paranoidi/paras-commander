package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/quickfilter"
	"github.com/paranoidi/paras-commander/internal/theme"
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

func (h commandsHost) CloseFileDialog() { h.app.dialogCtrl.CloseFileDialog() }

func (h commandsHost) FocusedFileDialogField() *dialog.FileDialogField {
	return h.app.dialogCtrl.FocusedField()
}

func (h commandsHost) RefreshAfterBackgroundCommand() { h.app.refreshAfterBackgroundCommand() }

func (h commandsHost) FilterOptions() quickfilter.Options { return h.app.quickFilterOptions() }

func (h commandsHost) Styles() theme.Theme { return h.app.styles }

// handleCommandsViewKey gives an open list-pane type-to-jump filter the key before the
// Esc-closes-view and vi-motion handling in commandsCtrl.HandleViewKey, like Dedup.
func (a *App) handleCommandsViewKey(event *tcell.EventKey) bool {
	if f := a.commandsCtrl.Filter(); a.model.CommandsView.FocusPane == 0 && f.UIActive() {
		if !filterRetainsKey(f, event, a.keys.Global) {
			a.commandsCtrl.CancelFilter()
		} else if a.handleListFilterKey(a.commandsCtrl, event) {
			return false
		}
	}
	return a.commandsCtrl.HandleViewKey(event)
}
