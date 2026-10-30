package app

import (
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// fileindexHost implements apphandler/fileindex.Host for *App.
type fileindexHost struct {
	appShellHost
}

func (h fileindexHost) ActivePanelSources() []string { return h.app.dialogCtrl.ActivePanelSources() }

func (h fileindexHost) InactivePanel() *panel.State { return h.app.inactivePanel() }

func (h fileindexHost) PrimaryPanel() *panel.State { return &h.app.model.Primary }

func (h fileindexHost) SecondaryPanel() *panel.State { return &h.app.model.Secondary }

func (h fileindexHost) NavigatePanelToPath(panelID int, path, selectName string) error {
	return h.app.navigatePanelToDirectory(panelID, path, selectName)
}

func (h fileindexHost) AddTaskJob(task *jobs.Task, sources []pathloc.Path) {
	h.app.jobsCtrl.AddTaskJob(task, sources)
}

func (h fileindexHost) ToggleFileIndexMenu() { h.app.toggleFileIndexMenu() }

func (h fileindexHost) RefreshMetaColumns() {
	h.app.metaCtrl.RefreshPanel(ui.PrimaryPanel)
	h.app.metaCtrl.RefreshPanel(ui.SecondaryPanel)
}
