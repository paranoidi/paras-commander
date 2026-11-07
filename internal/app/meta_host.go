package app

import (
	"errors"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// metaHost implements apphandler/meta.Host for *App.
type metaHost struct {
	appShellHost
}

func (h metaHost) PanelByID(panelID int) *panel.State { return h.app.panelByID(panelID) }

func (h metaHost) ResortPanel(panelID int) {
	p := h.app.panelByID(panelID)
	p.ApplySortFromDialog(p.Sort, h.app.panelViewportRows(panelID))
	h.app.invalidateMetaIdleSortPanel(panelID)
}

func (h metaHost) NoteMetaColumnResolved(panelID int) {
	h.app.maybeScheduleMetaIdleSort(panelID)
}

func (h metaHost) PanelMetaFit(panelID int) (ui.PanelMetaFit, bool) {
	a := h.app
	if a.model.ViewMode != ui.ViewBrowser {
		return ui.PanelMetaFit{}, false
	}
	w, ht := a.screen.Size()
	layout := a.layoutForTerminalSize(w, ht)
	if layout.TooSmall {
		return ui.PanelMetaFit{}, false
	}
	file, ok := a.panelFileFrame(layout, panelID)
	if !ok {
		return ui.PanelMetaFit{}, false
	}
	return ui.PanelMetaFitAt(file, &a.model, panelID, a.model.HideInactivePanel || a.autohideInactivePanel()), true
}

func (h metaHost) IconMetaRunning() string { return h.app.styles.IconMetaRunning() }
func (h metaHost) DialogMarkerWidth() int  { return dlg.MarkerWidth(h.app.styles) }

func (h metaHost) OpenFileInExternalEditor(path string) error {
	return h.app.openFileInExternalEditor(path)
}

func (h metaHost) SaveDefaultColumns(names []string) error {
	if !h.app.paths.CanPersist() {
		return errors.New("config.toml location is unknown; cannot save")
	}
	patch := map[string]interface{}{"fileindex": map[string]interface{}{"default_columns": names}}
	if err := h.app.persistPartial(patch); err != nil {
		return err
	}
	h.app.config.FileIndex.DefaultColumns = names
	return nil
}

func (h metaHost) MessageLogWrapCols() int { return h.app.messageLogWrapCols() }

func (h metaHost) AppendTransientMessageLines(banner string, lines []string, urgency ui.MessageUrgency) {
	h.app.appendTransientMessageLines(banner, lines, urgency)
}

func (h metaHost) ClearTransientMessage() { h.app.clearTransientMessage() }
