package app

import (
	"github.com/gdamore/tcell/v2"
	findctrl "github.com/paranoidi/paras-commander/internal/apphandler/find"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

type findHost struct {
	appShellHost
}

func (h findHost) PanelByID(panelID int) *panel.State { return h.app.panelByID(panelID) }

func (h findHost) ActiveViewportRows() int { return h.app.activeViewportRows() }

func (h findHost) InQuickFilterUI() bool { return h.app.inQuickFilterUI() }

func (h findHost) NavigatePanelToPath(panelID int, path, selectName string) error {
	return h.app.navigatePanelToDirectory(panelID, path, selectName)
}

func (h findHost) HandleScrollingQueryKey(ev *tcell.EventKey, inputFocused bool, edit findctrl.ScrollingQueryEdit) bool {
	inner, ok := edit.Value().(scrollingQueryEdit)
	if !ok {
		return false
	}
	return h.app.handleScrollingQueryKey(ev, inputFocused, inner)
}

func (h findHost) FindDialogScrollingQuery(st *dialog.FindDialogState, width int, onChange func()) findctrl.ScrollingQueryEdit {
	return findctrl.NewScrollingQueryEdit(findDialogScrollingQuery(st, width, onChange))
}

func (h findHost) FindDialogQueryWidth() int { return h.app.findDialogQueryWidth() }

func (h findHost) DiskUsageIgnore() diskusage.ShouldIgnoreFolder { return h.app.diskUsageIgnore }

func (h findHost) GitignoreCache() *gitignore.Cache { return h.app.gitignoreCache }

func (h findHost) PanelViewportRows(panelID int) int { return h.app.panelViewportRows(panelID) }

func (h findHost) OpenGroupSelectDialog(mode findctrl.GroupSelectMode, forFind bool) {
	context := "panel"
	if forFind {
		context = "find"
	}
	h.app.openGroupSelect(string(mode), context)
}
