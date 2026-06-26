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

func (h compareHost) CompareMenuDefinitions() []menu.Definition {
	return h.app.browserMenuDefinitions()
}

func (h compareHost) BrowserMenuDefinitions() []menu.Definition {
	return h.app.browserMenuDefinitions()
}
