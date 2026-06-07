package find

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Host supplies cross-cutting app services the find handler cannot import from internal/app.
type Host interface {
	host.LayoutHost
	host.MessageHost

	PanelByID(panelID int) *panel.State
	ActivePanel() *panel.State
	ActiveViewportRows() int
	InQuickFilterUI() bool
	NavigatePanelToDirectory(panelID int, path, message string) error
	HandleScrollingQueryKey(ev *tcell.EventKey, inputFocused bool, edit any) bool
	FindDialogScrollingQuery(st *ui.FindDialogState, width int, onChange func()) any
	FindDialogQueryWidth() int
	DiskUsageIgnore() diskusage.ShouldIgnoreFolder
	GitignoreCache() *gitignore.Cache
	PanelViewportRows(panelID int) int
	OpenGroupSelectDialog(mode string, forFind bool)
}
