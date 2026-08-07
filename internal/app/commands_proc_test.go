package app

import (
	"time"

	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"

	"testing"
)

// waitCommandRowRunning polls until CommandsList[idx] reaches CommandRunRunning, or fails the test.
func waitCommandRowRunning(t *testing.T, app *App, idx int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		app.commandsMu.RLock()
		running := idx < len(app.model.CommandsList) && app.model.CommandsList[idx].Phase == ui.CommandRunRunning
		app.commandsMu.RUnlock()
		if running {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for command row to start running")
}

// waitCommandsDoneWithin is waitCommandsDone with a caller-chosen deadline, so tests can assert
// that terminate/kill actually cut a long-running command short instead of letting it run out.
func waitCommandsDoneWithin(t *testing.T, app *App, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !app.commandsCtrl.HasRunning() {
			app.commandsMu.RLock()
			allDone := len(app.model.CommandsList) > 0
			for _, e := range app.model.CommandsList {
				if e.Phase != ui.CommandRunDone {
					allDone = false
					break
				}
			}
			app.commandsMu.RUnlock()
			if allDone {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for command run to finish")
}

// TestCommandsKillStopsRunningSubprocess covers the process-group signal: sh does not
// exec-replace itself for a script file, so sleep runs as sh's child. Signaling only sh's
// pid would leave sleep running, holding the output pipe open until cmd.WaitDelay (5s)
// force-closes it — this must finish well under that.
func TestCommandsKillStopsRunningSubprocess(t *testing.T) {
	root := t.TempDir()
	writeExecutableScript(t, root, "runme.sh", "#!/bin/sh\nsleep 30\n")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	selectEntryByName(t, app, "runme.sh")

	app.dispatch(keymap.ActionNavOpen)
	waitCommandRowRunning(t, app, 0)

	app.model.CommandsView.Selected = 0
	app.dispatch(keymap.ActionCommandsKill)

	waitCommandsDoneWithin(t, app, 8*time.Second)

	e := app.model.CommandsList[0]
	if e.ExitCode == 0 {
		t.Fatalf("ExitCode = %d, want nonzero (killed)", e.ExitCode)
	}
}

func TestCommandsTerminateSendsSIGTERM(t *testing.T) {
	root := t.TempDir()
	writeExecutableScript(t, root, "runme.sh", "#!/bin/sh\ntrap 'exit 7' TERM\nsleep 30 &\nwait\n")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	selectEntryByName(t, app, "runme.sh")

	app.dispatch(keymap.ActionNavOpen)
	waitCommandRowRunning(t, app, 0)

	app.model.CommandsView.Selected = 0
	app.dispatch(keymap.ActionCommandsTerminate)

	waitCommandsDoneWithin(t, app, 8*time.Second)

	e := app.model.CommandsList[0]
	if e.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7 (from SIGTERM trap)", e.ExitCode)
	}
}

func TestCommandsKillNoOpWhenNoRowSelected(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.model.ViewMode = ui.ViewCommands
	app.model.CommandsView.Selected = 0

	if app.dispatch(keymap.ActionCommandsKill) {
		t.Fatal("dispatch(ActionCommandsKill) should not request quit")
	}
	if len(app.model.MessageLog) == 0 {
		t.Fatal("expected a transient message when no running row is selected")
	}
}
