package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// Host key prompt button IDs. The dialog has no OK / Cancel buttons: Esc (the central cancel
// chord) rejects the key.
const (
	HostKeyAccept = dlg.ID("accept")
	HostKeySave   = dlg.ID("save")
	HostKeyReject = dlg.ID("reject")
)

// HostKeyView is the host key prompt's widget spec: the host and key details above Accept /
// Save / Reject. Plain a / s / r press the buttons. It is as wide as the screen allows up to 72
// columns, and never below 44 where the screen has room.
func HostKeyView(s HostKeyDialogState) dlg.Spec {
	lines := []string{"Unknown host key for:", s.Host, "Type: " + s.KeyType, "SHA256: " + s.Fingerprint}
	items := make([]dlg.Item, len(lines))
	for i, line := range lines {
		items[i] = dlg.Item{Kind: dlg.KindLabel, Label: line}
	}
	return dlg.Spec{
		Title: "Host key", Width: 72, MinWidth: 44, Unclamped: true, PlainButtonKeys: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons: []dlg.Button{
			{ID: HostKeyAccept, Label: "Accept", Mnemonic: 'A'},
			{ID: HostKeySave, Label: "Save", Mnemonic: 'S'},
			{ID: HostKeyReject, Label: "Reject", Mnemonic: 'R'},
		},
	}
}
