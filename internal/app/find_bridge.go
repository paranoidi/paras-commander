package app

import (
	"github.com/gdamore/tcell/v2"
	findctrl "github.com/paranoidi/paras-commander/internal/apphandler/find"
)

func (a *App) openFindDialog(panelID int) { a.findCtrl.OpenDialog(panelID) }

func (a *App) closeFindDialog() { a.findCtrl.CloseDialog() }

func (a *App) handleFindDialogKey(event *tcell.EventKey) { a.findCtrl.HandleDialogKey(event) }

func (a *App) pollFindUpdates(payload findctrl.WakePayload) bool {
	return a.findCtrl.PollUpdates(payload)
}

func (a *App) activateFindDialogOK() { a.findCtrl.ActivateDialogOK() }

func (a *App) toggleFindStayOnVolume() { a.findCtrl.ToggleStayOnVolume() }

func (a *App) toggleFindSearchOnlySelections() { a.findCtrl.ToggleSearchOnlySelections() }

func (a *App) navigateFindCursor() { a.findCtrl.NavigateFindCursor() }
