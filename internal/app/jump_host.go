package app

import (
	jumpctrl "github.com/paranoidi/paras-commander/internal/apphandler/jump"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

type jumpHost struct {
	appShellHost
}

func (h jumpHost) PanelByID(panelID int) *panel.State { return h.app.panelByID(panelID) }

func (h jumpHost) PanelViewportRows(panelID int) int { return h.app.panelViewportRows(panelID) }

func (h jumpHost) NavigatePanelToPath(panelID int, path, selectName string) error {
	return h.app.navigatePanelToDirectory(panelID, path, selectName)
}

func (h jumpHost) Styles() theme.Theme { return h.app.styles }

func (h jumpHost) GitignoreCache() *gitignore.Cache { return h.app.gitignoreCache }

func (h jumpHost) PinTogglePath(name, path string, isDir bool) {
	h.app.pinCtrl.TogglePath(name, path, isDir)
}

var _ jumpctrl.Host = jumpHost{}

// openJumpDialog is the one entry for panel.jump-dialog: Jump to directory on panelID, once the
// index opens when the panel is a fileindex:// one.
func (a *App) openJumpDialog(panelID int) {
	if a.panelByID(panelID).Path.Scheme() == pathloc.SchemeFileIndex {
		a.fileindexCtrl.WithIndex(func() { a.jumpCtrl.Open(panelID) })
		return
	}
	a.jumpCtrl.Open(panelID)
}
