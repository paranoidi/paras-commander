package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// Quit confirmation button IDs. The dialog has no OK / Cancel buttons: Esc (the central cancel
// chord) closes it like Stay.
const (
	QuitStay = dlg.ID("stay")
	QuitQuit = dlg.ID("quit")
)

// QuitConfirmView is the quit confirmation's widget spec: the warning copy (WarnLine1/2 override
// the defaults) above Stay / Quit Anyway.
func QuitConfirmView(s QuitConfirmState) dlg.Spec {
	line1, line2 := s.WarnLine1, s.WarnLine2
	if line1 == "" {
		line1 = "Active jobs are queued or running."
	}
	if line2 == "" {
		line2 = "Quitting will interrupt these operations."
	}
	return dlg.Spec{
		Title: "Quit", Width: 50, Unclamped: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			{Kind: dlg.KindLabel, Label: line1},
			{Kind: dlg.KindLabel, Label: line2},
		}}}},
		Buttons: []dlg.Button{
			{ID: QuitStay, Label: "Stay", Mnemonic: 'S'},
			{ID: QuitQuit, Label: "Quit Anyway", Mnemonic: 'Q'},
		},
	}
}
