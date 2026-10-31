package app

import (
	findctrl "github.com/paranoidi/paras-commander/internal/apphandler/find"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
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

func (h fileindexHost) PanelByID(panelID int) *panel.State { return h.app.panelByID(panelID) }

func (h fileindexHost) PanelViewportRows(panelID int) int { return h.app.panelViewportRows(panelID) }

func (h fileindexHost) Styles() theme.Theme { return h.app.styles }

func (h fileindexHost) OpenFullscreenFilePreviewAt(path string, isDir bool) error {
	return h.app.previewCtrl.OpenFullscreenFilePreviewAt(path, isDir)
}

// openFindDialog is the one entry for panel.find-dialog and panel.jump-dialog: the find-in-index
// dialog on a fileindex:// panel, Find everywhere else.
func (a *App) openFindDialog(panelID int, jump bool) {
	if a.panelByID(panelID).Path.Scheme() == pathloc.SchemeFileIndex {
		a.fileindexCtrl.OpenFindDialog(panelID, jump)
		return
	}
	if jump {
		a.findCtrl.OpenJumpDialog(panelID)
		return
	}
	a.findCtrl.OpenDialog(panelID)
}

func (h fileindexHost) PinTogglePath(name, path string, isDir bool) {
	h.app.pinCtrl.TogglePath(name, path, isDir)
}

func (h fileindexHost) OpenGroupSelectDialog(mode findctrl.GroupSelectMode) {
	h.app.openGroupSelect(string(mode), "index-find")
}
