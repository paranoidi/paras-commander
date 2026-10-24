package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// UndoConfirmState holds the confirmation shown before a job's undo is queued.
type UndoConfirmState struct {
	Open  bool
	Focus dlg.ID // UndoYes or UndoNo (the default)
	JobID string
	// Line1 names the job and step count; Line2 says what undo will do.
	Line1 string
	Line2 string
}
