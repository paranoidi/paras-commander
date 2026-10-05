package dialog

import "context"

// CommandOutputDialogState holds the state for the user-command output dialog.
type CommandOutputDialogState struct {
	Open       bool
	Title      string
	Lines      []string // stdout pre-split on \n
	Scroll     int      // first visible line index
	PrefWidth  string   // from config, e.g. "80%"
	PrefHeight string   // from config, e.g. "50%"

	// Running is true while the command is still executing; RunID is the Commands-view row ID
	// of the run so a late result is dropped if the dialog was backgrounded or closed. Cancel
	// kills the running command. Focus is the running-state button (0 Background, 1 Cancel).
	// Queued is true while a running command still waits for a worker-pool slot.
	Running bool
	Queued  bool
	RunID   string
	Cancel  context.CancelFunc
	Focus   int
}
