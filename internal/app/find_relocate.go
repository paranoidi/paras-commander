package app

import (
	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// handleFindRelocateKey drives the parked-find relocate question.
func (a *App) handleFindRelocateKey(event *tcell.EventKey) {
	d := &a.model.FindRelocate
	if ev, ok := dlg.HandleKey(dialog.FindRelocateView(*d), &d.Focus, event); ok {
		a.applyFindRelocateEvent(ev)
	}
}

// applyFindRelocateEvent answers the parked-find relocate question (key or click).
func (a *App) applyFindRelocateEvent(ev dlg.Event) {
	if ev.ID != "" {
		a.answerFindRelocate(ev.ID)
	}
}

// answerFindRelocate closes the question; Restore shows the parked search, New search here
// starts over on the panel Find was opened from, anything else (Cancel) leaves it parked.
func (a *App) answerFindRelocate(id dlg.ID) {
	s := a.model.FindRelocate
	a.model.FindRelocate = dialog.FindRelocateState{}
	switch {
	case id == dialog.FindRelocateRestore && s.Index:
		a.fileindexCtrl.RestoreParkedFind()
	case id == dialog.FindRelocateRestore:
		a.findCtrl.RestoreParked()
	case id == dialog.FindRelocateNew && s.Unique:
		a.fileindexCtrl.OpenFreshUnique(s.PanelID)
	case id == dialog.FindRelocateNew && s.Index:
		a.fileindexCtrl.OpenFreshFind(s.PanelID, s.Jump)
	case id == dialog.FindRelocateNew:
		a.findCtrl.OpenFresh(s.PanelID, s.Jump)
	}
}
