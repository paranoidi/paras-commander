package dialog

// DedupProgressDialogState is the modal shown while find-duplicates scans a directory.
type DedupProgressDialogState struct {
	Open        bool
	ButtonFocus int // confirm gate: 0=OK, 1=Cancel; other phases: 0=Cancel only
}
