package app

import (
	"slices"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/statuscmds"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// openStatusCommandsDialog re-reads status.toml (so manual edits show up) and opens the
// dialog choosing which status commands are drawn in the menu bar.
func (a *App) openStatusCommandsDialog() {
	set, err := statuscmds.Load(a.statusCommandsPath())
	if err != nil {
		a.setErrorMessage("Status commands", err)
		return
	}
	rows := make([]dialog.StatusCommandsDialogRow, len(set.Defs))
	for i, d := range set.Defs {
		rows[i] = dialog.StatusCommandsDialogRow{Name: d.Name, Label: d.Label, Shown: slices.Contains(set.Show, d.Name)}
	}
	a.model.StatusCommandsDialog = dialog.StatusCommandsDialogState{Open: true, Rows: rows}
}

func (a *App) closeStatusCommandsDialog() {
	a.model.StatusCommandsDialog.Open = false
}

// applyStatusCommandsDialog saves the checked commands as status.toml's show list and
// restarts the status commands. Commands that stay shown keep their order in the file;
// newly checked ones follow in dialog order.
func (a *App) applyStatusCommandsDialog() {
	path := a.statusCommandsPath()
	set, err := statuscmds.Load(path)
	if err != nil {
		a.setErrorMessage("Status commands", err)
		return
	}
	checked := map[string]bool{}
	for _, r := range a.model.StatusCommandsDialog.Rows {
		checked[r.Name] = r.Shown
	}
	var show []string
	for _, name := range set.Show {
		if checked[name] {
			show = append(show, name)
		}
	}
	for _, r := range a.model.StatusCommandsDialog.Rows {
		if r.Shown && !slices.Contains(show, r.Name) {
			show = append(show, r.Name)
		}
	}
	a.closeStatusCommandsDialog()
	if slices.Equal(show, set.Show) {
		return
	}
	if err := statuscmds.SaveShow(path, show); err != nil {
		a.setErrorMessage("Status commands", err)
		return
	}
	a.restartStatusCommands()
	a.setTransientMessage("Status commands saved", ui.MessageUrgencyInfo)
}

func (a *App) handleStatusCommandsDialogKey(event *tcell.EventKey) {
	st := &a.model.StatusCommandsDialog
	if ev, ok := dlg.HandleKey(dialog.StatusCommandsDialogView(*st), &st.Focus, event); ok {
		a.applyStatusCommandsDialogEvent(ev)
	}
}

// applyStatusCommandsDialogEvent acts on a widget event of the status-commands dialog (key
// or click).
func (a *App) applyStatusCommandsDialogEvent(ev dlg.Event) {
	switch ev.ID {
	case dlg.OK:
		a.applyStatusCommandsDialog()
	case dlg.Cancel:
		a.closeStatusCommandsDialog()
	default:
		a.model.StatusCommandsDialog.Apply(ev)
	}
}
