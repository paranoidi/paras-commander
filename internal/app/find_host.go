package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type findHost struct{ app *App }

func (h findHost) PanelByID(panelID int) *panel.State { return h.app.panelByID(panelID) }

func (h findHost) ActivePanel() *panel.State { return h.app.activePanel() }

func (h findHost) ActiveViewportRows() int { return h.app.activeViewportRows() }

func (h findHost) InQuickFilterUI() bool { return h.app.inQuickFilterUI() }

func (h findHost) NavigatePanelToDirectory(panelID int, path, message string) error {
	return h.app.navigatePanelToDirectory(panelID, path, message)
}

func (h findHost) SetTransientMessage(text string, urgency ui.MessageUrgency) {
	h.app.setTransientMessage(text, urgency)
}

func (h findHost) SetErrorMessage(title string, err error) { h.app.setErrorMessage(title, err) }

func (h findHost) HandleScrollingQueryKey(ev *tcell.EventKey, inputFocused bool, edit any) bool {
	return h.app.handleScrollingQueryKey(ev, inputFocused, edit.(scrollingQueryEdit))
}

func (h findHost) FindDialogScrollingQuery(st *ui.FindDialogState, width int, onChange func()) any {
	return findDialogScrollingQuery(st, width, onChange)
}

func (h findHost) FindDialogQueryWidth() int { return h.app.findDialogQueryWidth() }

func (h findHost) DiskUsageIgnore() diskusage.ShouldIgnoreFolder { return h.app.diskUsageIgnore }

func (h findHost) LayoutForTerminalSize(w, height int) ui.Layout {
	return h.app.layoutForTerminalSize(w, height)
}

func (h findHost) PanelViewportRows(panelID int) int { return h.app.panelViewportRows(panelID) }
