package app

import (
	"path/filepath"
)

type duplicateFocusPending struct {
	panelID int
	listDir string
	name    string
}

func (a *App) scheduleDuplicateFocus(panelID int, listDir, name string) {
	a.duplicateFocus = duplicateFocusPending{
		panelID: panelID,
		listDir: listDir,
		name:    name,
	}
}

func (a *App) applyDuplicateFocusPending() {
	f := a.duplicateFocus
	if f.name == "" {
		return
	}
	p := a.panelByID(f.panelID)
	if p == nil {
		a.duplicateFocus = duplicateFocusPending{}
		return
	}
	if filepath.Clean(p.PathString()) != filepath.Clean(f.listDir) {
		a.duplicateFocus = duplicateFocusPending{}
		return
	}
	if p.SelectVisibleEntryCentered(f.name, a.panelViewportRows(f.panelID)) {
		a.duplicateFocus = duplicateFocusPending{}
	}
}
