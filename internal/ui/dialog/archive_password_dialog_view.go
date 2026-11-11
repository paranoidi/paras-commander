package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// ArchivePasswordInput is the archive password prompt's masked input widget ID. The buttons are
// dlg.OK and dlg.Cancel.
const ArchivePasswordInput = dlg.ID("password")

// ArchivePasswordView is the encrypted-archive prompt's widget spec: the archive, a "Wrong
// password." line after a rejected one, the masked password input, above OK / Cancel (Cancel
// skips the archive). It is as wide as the screen allows up to 64 columns, and never below 44
// where the screen has room.
func ArchivePasswordView(s ArchivePasswordDialogState) dlg.Spec {
	head := []dlg.Item{
		{Kind: dlg.KindLabel, Label: "Encrypted archive:"},
		{Kind: dlg.KindLabel, Label: s.Path, Fit: dlg.FitMiddle},
	}
	if s.BadPassword {
		head = append(head, dlg.Item{Kind: dlg.KindLabel, Label: "Wrong password.", Style: dlg.TextError})
	}
	head = append(head, dlg.Item{Kind: dlg.KindSeparator})
	return dlg.Spec{
		Title: "Archive password", Width: 64, MinWidth: 44, Unclamped: true,
		Groups:  []dlg.Group{{Columns: [][]dlg.Item{head}}, {Columns: [][]dlg.Item{maskedPasswordItems(ArchivePasswordInput, s.Password)}}},
		Buttons: []dlg.Button{dlg.OKButton, dlg.CancelButton},
	}
}
