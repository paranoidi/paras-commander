package app

import (
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

type compareHost struct {
	appShellHost
}

func (h compareHost) NavigatePanelToPath(panelID int, path string, selectName string) error {
	return h.app.navigatePanelToDirectory(panelID, path, selectName)
}

func (h compareHost) TogglePanelSelection(panelID int, path string) bool {
	p := h.app.panelByID(panelID)
	_, conflicts := p.TogglePathSelection(path)
	return conflicts
}

func (h compareHost) SetTransientMessage(text string, urgency ui.MessageUrgency) {
	h.app.setTransientMessage(text, urgency)
}

func (h compareHost) ClearTransientMessage() {
	h.app.clearTransientMessage()
}

func (h compareHost) CompareMenuDefinitions() []menu.Definition {
	return h.app.compareMenuDefinitions()
}

func (a *App) compareMenuDefinitions() []menu.Definition {
	return menu.CompareDefinitions(a.keys.Global, a.keys.Compare, a.model.CompareView.IgnoreEmpty)
}

func (h compareHost) BrowserMenuDefinitions() []menu.Definition {
	return h.app.browserMenuDefinitions()
}
