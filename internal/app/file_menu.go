package app

import (
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func (a *App) dispatchFileMenuItem(item menu.Item) {
	switch item.Action {
	case keymap.ActionFileExtract,
		keymap.ActionFileDelete,
		keymap.ActionFileMkdir,
		keymap.ActionFileChmod,
		keymap.ActionFileChown,
		keymap.ActionFileSymlink,
		keymap.ActionFileHardlink,
		keymap.ActionFileView,
		keymap.ActionFileQuickView,
		keymap.ActionMenuFileViewPath,
		keymap.ActionFileEdit,
		keymap.ActionMenuFileRelativeSymlink,
		keymap.ActionMenuFileEditSymlink,
		keymap.ActionMenuFileChattr:
		a.dispatch(item.Action)
	default:
		a.setUnsupportedMessage(item.Label)
	}
}
