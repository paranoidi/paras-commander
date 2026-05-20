package dialog

// HostKeyDialogState prompts when an SSH host key is unknown or changed.
type HostKeyDialogState struct {
	Open        bool
	Host        string
	KeyType     string
	Fingerprint string
	Focus       int // 0=accept, 1=accept & save, 2=reject
}
