package app

import (
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// metaHost implements apphandler/meta.Host for *App.
type metaHost struct {
	appShellHost
}

func (h metaHost) PanelByID(panelID int) *panel.State { return h.app.panelByID(panelID) }

func (h metaHost) SymbolMetaRunning() string { return h.app.styles.SymbolMetaRunning() }

func (h metaHost) OpenFileInExternalEditor(path string) error {
	return h.app.openFileInExternalEditor(path)
}

func (h metaHost) MessageLogWrapCols() int { return h.app.messageLogWrapCols() }

func (h metaHost) AppendTransientMessageLines(banner string, lines []string, urgency ui.MessageUrgency) {
	h.app.appendTransientMessageLines(banner, lines, urgency)
}

func (h metaHost) ClearTransientMessage() { h.app.clearTransientMessage() }

func (h metaHost) Render() { h.app.render() }
