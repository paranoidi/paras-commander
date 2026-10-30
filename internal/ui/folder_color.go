package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
)

// folderColorFor returns the folder-colour lookup for a panel, nil for remote panels (the colours
// describe local directories only).
func (m Model) folderColorFor(state panel.State) func(string) (tcell.Color, bool) {
	if state.Path.IsRemote() {
		return nil
	}
	return m.FolderColor
}
