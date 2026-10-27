package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

const statusCommandPrefix = "status:"

// StatusCommandsDialogRow is one status command offered in the status-commands dialog.
type StatusCommandsDialogRow struct {
	Name  string // status.toml name (stable identifier)
	Label string // checkbox text
	Shown bool   // checkbox state
}

// StatusCommandsDialogState is the renderable state for the dialog choosing which status
// commands are drawn at the top-left of the menu bar.
type StatusCommandsDialogState struct {
	Open bool
	Rows []StatusCommandsDialogRow
	// Focus is the focused widget of StatusCommandsDialogView; empty focuses the first checkbox.
	Focus dlg.ID
}

// StatusCommandsDialogView is the status-commands dialog's widget spec: one checkbox per
// status command, then OK / Cancel. Rows carry no mnemonics since labels are user-defined.
func StatusCommandsDialogView(s StatusCommandsDialogState) dlg.Spec {
	items := make([]dlg.Item, len(s.Rows))
	for i, r := range s.Rows {
		items[i] = dlg.Item{ID: dlg.ID(statusCommandPrefix + r.Name), Kind: dlg.KindCheckbox, Label: r.Label, On: r.Shown}
	}
	return dlg.Spec{
		Title: "Status commands", Width: 40, MinWidth: 24, MustFit: true, PlainButtonKeys: true,
		Groups:  []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons: []dlg.Button{dlg.OKButton, dlg.CancelButton},
	}
}

// Apply toggles the checkbox an activation from dlg.HandleKey names. OK and Cancel are the
// caller's to handle.
func (s *StatusCommandsDialogState) Apply(ev dlg.Event) {
	for i := range s.Rows {
		if ev.ID == dlg.ID(statusCommandPrefix+s.Rows[i].Name) {
			s.Rows[i].Shown = !s.Rows[i].Shown
			return
		}
	}
}

// DrawStatusCommandsDialog paints the status-commands dialog from its widget spec.
func DrawStatusCommandsDialog(screen tcell.Screen, layout Layout, state StatusCommandsDialogState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, StatusCommandsDialogView(state), state.Focus, styles)
}
