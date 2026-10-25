package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// DedupProgressDialogState is the modal shown while find-duplicates scans a directory. Focus is
// dlg.OK or dlg.Cancel; an unset or stale focus (the OK button is gone after the confirm gate)
// is the first button.
type DedupProgressDialogState struct {
	Open  bool
	Focus dlg.ID
}
