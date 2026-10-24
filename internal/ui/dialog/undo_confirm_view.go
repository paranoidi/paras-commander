package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// Undo confirmation button IDs. The dialog has no OK / Cancel buttons: Yes queues the undo, No
// (or Esc) leaves the job alone.
const (
	UndoYes = dlg.ID("yes")
	UndoNo  = dlg.ID("no")
)

// UndoConfirmView is the undo confirmation's widget spec: two message lines above Yes / No.
func UndoConfirmView(s UndoConfirmState) dlg.Spec {
	return dlg.Spec{
		Title: "Undo job", Width: 56, Unclamped: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			{Kind: dlg.KindLabel, Label: s.Line1},
			{Kind: dlg.KindLabel, Label: s.Line2},
		}}}},
		Buttons: []dlg.Button{
			{ID: UndoYes, Label: "Yes", Mnemonic: 'Y'},
			{ID: UndoNo, Label: "No", Mnemonic: 'N'},
		},
	}
}
