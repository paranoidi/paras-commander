package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// StashRestoreDialogState holds the selection-stash restore conflict dialog.
type StashRestoreDialogState struct {
	Open  bool
	Focus dlg.ID // a Stash* button ID (StashReplace is the default) or dlg.Cancel
}
