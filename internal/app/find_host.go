package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type findHost struct {
	appShellHost
}

func (h findHost) PanelByID(panelID int) *panel.State { return h.app.panelByID(panelID) }

func (h findHost) ActiveViewportRows() int { return h.app.activeViewportRows() }

func (h findHost) InQuickFilterUI() bool { return h.app.inQuickFilterUI() }

func (h findHost) NavigatePanelToDirectory(panelID int, path, message string) error {
	return h.app.navigatePanelToDirectory(panelID, path, message)
}

func (h findHost) HandleScrollingQueryKey(ev *tcell.EventKey, inputFocused bool, edit any) bool {
	return h.app.handleScrollingQueryKey(ev, inputFocused, edit.(scrollingQueryEdit))
}

func (h findHost) FindDialogScrollingQuery(st *ui.FindDialogState, width int, onChange func()) any {
	return findDialogScrollingQuery(st, width, onChange)
}

func (h findHost) FindDialogQueryWidth() int { return h.app.findDialogQueryWidth() }

func (h findHost) DiskUsageIgnore() diskusage.ShouldIgnoreFolder { return h.app.diskUsageIgnore }

func (h findHost) GitignoreCache() *gitignore.Cache { return h.app.gitignoreCache }

func (h findHost) PanelViewportRows(panelID int) int { return h.app.panelViewportRows(panelID) }

func (h findHost) OpenGroupSelectDialog(mode string, forFind bool) {
	context := "panel"
	if forFind {
		context = "find"
	}
	h.app.openGroupSelect(mode, context)
}
