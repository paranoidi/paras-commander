package app

import (
	"path/filepath"
)

type copyHereFocusPending struct {
	panelID int
	listDir string
	name    string
}

func (a *App) scheduleCopyHereFocus(panelID int, listDir, name string) {
	a.copyHereFocus = copyHereFocusPending{
		panelID: panelID,
		listDir: listDir,
		name:    name,
	}
}

func (a *App) applyCopyHereFocusPending() {
	f := a.copyHereFocus
	if f.name == "" {
		return
	}
	p := a.panelByID(f.panelID)
	if p == nil {
		a.copyHereFocus = copyHereFocusPending{}
		return
	}
	if filepath.Clean(p.PathString()) != filepath.Clean(f.listDir) {
		a.copyHereFocus = copyHereFocusPending{}
		return
	}
	if p.SelectVisibleEntryCentered(f.name, a.panelViewportRows(f.panelID)) {
		a.copyHereFocus = copyHereFocusPending{}
	}
}
