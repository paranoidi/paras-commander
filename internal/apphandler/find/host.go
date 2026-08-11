package find

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// Host supplies cross-cutting app services the find handler cannot import from internal/app.
type Host interface {
	host.LayoutHost
	host.MessageHost

	PanelByID(panelID int) *panel.State
	ActivePanel() *panel.State
	ActiveViewportRows() int
	InQuickFilterUI() bool
	host.PanelNavigationHost
	HandleScrollingQueryKey(ev *tcell.EventKey, inputFocused bool, edit ScrollingQueryEdit) bool
	FindDialogScrollingQuery(st *dialog.FindDialogState, width int, onChange func()) ScrollingQueryEdit
	FindDialogQueryWidth() int
	DiskUsageIgnore() diskusage.ShouldIgnoreFolder
	GitignoreCache() *gitignore.Cache
	PanelViewportRows(panelID int) int
	OpenGroupSelectDialog(mode GroupSelectMode, forFind bool)
	OpenFullscreenFilePreviewAt(path string) error
}
