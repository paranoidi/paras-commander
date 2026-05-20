package find

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Host supplies cross-cutting app services the find handler cannot import from internal/app.
type Host interface {
	PanelByID(panelID int) *panel.State
	ActivePanel() *panel.State
	ActiveViewportRows() int
	InQuickFilterUI() bool
	NavigatePanelToDirectory(panelID int, path, message string) error
	SetTransientMessage(text string, urgency ui.MessageUrgency)
	SetErrorMessage(title string, err error)
	HandleScrollingQueryKey(ev *tcell.EventKey, inputFocused bool, edit any) bool
	FindDialogScrollingQuery(st *ui.FindDialogState, width int, onChange func()) any
	FindDialogQueryWidth() int
	DiskUsageIgnore() diskusage.ShouldIgnoreFolder
	GitignoreCache() *gitignore.Cache
	LayoutForTerminalSize(w, h int) ui.Layout
	PanelViewportRows(panelID int) int
}
