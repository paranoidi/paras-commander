package dialog

// CommandOutputDialogState holds the state for the user-command output dialog.
type CommandOutputDialogState struct {
	Open       bool
	Title      string
	Lines      []string // stdout pre-split on \n
	Scroll     int      // first visible line index
	PrefWidth  string   // from config, e.g. "80%"
	PrefHeight string   // from config, e.g. "50%"
}
