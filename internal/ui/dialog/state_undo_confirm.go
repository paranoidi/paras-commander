package dialog

// UndoConfirmState holds the confirmation shown before a job's undo is queued.
type UndoConfirmState struct {
	Open  bool
	Focus int // 0=Yes, 1=No (default)
	JobID string
	// Line1 names the job and step count; Line2 says what undo will do.
	Line1 string
	Line2 string
}
