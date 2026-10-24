package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// HostKeyDialogState prompts when an SSH host key is unknown or changed.
type HostKeyDialogState struct {
	Open        bool
	Host        string
	KeyType     string
	Fingerprint string
	Focus       dlg.ID // HostKeyAccept (the default), HostKeySave or HostKeyReject
}
