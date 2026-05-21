package dialog

// StashRestoreDialogState holds the selection-stash restore conflict dialog.
type StashRestoreDialogState struct {
	Open  bool
	Focus int // 0=Replace, 1=Merge, 2=Drop stash, 3=Drop all
}
