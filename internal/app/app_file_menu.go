package app

import (
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func (a *App) dispatchFileMenuItem(item menu.Item) {
	switch item.Action {
	case keymap.ActionFileDelete,
		keymap.ActionFileMkdir,
		keymap.ActionFileChmod,
		keymap.ActionFileChown,
		keymap.ActionFileSymlink,
		keymap.ActionFileHardlink,
		keymap.ActionBookmarkOpen,
		keymap.ActionFileView,
		keymap.ActionMenuFileViewPath,
		keymap.ActionMenuFileFilteredView,
		keymap.ActionFileEdit,
		keymap.ActionMenuFileRelativeSymlink,
		keymap.ActionMenuFileEditSymlink,
		keymap.ActionMenuFileAdvancedChown,
		keymap.ActionMenuFileChattr:
		a.dispatch(item.Action)
	default:
		a.setUnsupportedMessage(item.Label)
	}
}
