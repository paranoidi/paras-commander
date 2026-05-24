package ui

import "strings"

// CommandRunPhase is the lifecycle state of one subprocess row.
type CommandRunPhase int

const (
	CommandRunPending CommandRunPhase = iota
	CommandRunRunning
	CommandRunDone
)

// CommandRunKind labels how a row was produced (future run modes share Commands view).
type CommandRunKind string

const (
	// CommandRunKindRunForEach is one invocation per source path with argv prefix + path.
	CommandRunKindRunForEach CommandRunKind = "run_for_each"
	// CommandRunKindUserMenu is a single argv invocation from the F2 user menu (menu.toml).
	CommandRunKindUserMenu CommandRunKind = "user_menu"
	// CommandRunKindFileExecute is Enter on a POSIX-executable regular file in the file browser.
	CommandRunKindFileExecute CommandRunKind = "file_execute"
)

// CommandRunEntry is one row in the Commands screen list plus captured output.
type CommandRunEntry struct {
	ID              string
	Kind            CommandRunKind
	UserCommandLine string
	TargetPath      string
	Phase           CommandRunPhase
	ExitCode        int // set when Phase == CommandRunDone; -1 while pending/running or unknown
	Stdout          string
	Stderr          string
	ErrorMsg        string // launch failure or context cancellation
}

// CommandStderrDisplay returns stderr text for the Commands view, falling back to ErrorMsg when
// both stdout and stderr are empty (e.g. launch failures before the subprocess starts).
func CommandStderrDisplay(e CommandRunEntry) string {
	if strings.TrimSpace(e.Stderr) != "" {
		return e.Stderr
	}
	if strings.TrimSpace(e.Stdout) == "" {
		if msg := strings.TrimSpace(e.ErrorMsg); msg != "" {
			return msg
		}
	}
	return e.Stderr
}

// CommandsViewState tracks focus and scrolling for the Commands screen.
type CommandsViewState struct {
	Selected     int
	FocusPane    int // 0=list, 1=stdout, 2=stderr
	ListScroll   int
	StdoutScroll int
	StderrScroll int
}
