package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func writeExecutableScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func waitCommandsDone(t *testing.T, app *App) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
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
	t.Fatal("timed out waiting for command run")
}

func selectEntryByName(t *testing.T, app *App, name string) {
	t.Helper()
	p := app.activePanel()
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Name == name {
			p.Cursor = i
			return
		}
	}
	t.Fatalf("entry %q not found in panel", name)
}

func TestNavOpenRunsExecutableInCommandsView(t *testing.T) {
	root := t.TempDir()
	marker := "PARAS_EXEC_MARKER"
	writeExecutableScript(t, root, "runme.sh", "#!/bin/sh\necho "+marker+"\n")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	selectEntryByName(t, app, "runme.sh")

	app.dispatch(keymap.ActionNavOpen)
	waitCommandsDone(t, app)

	if app.model.ViewMode != ui.ViewCommands {
		t.Fatalf("ViewMode = %v, want ViewCommands", app.model.ViewMode)
	}
	if len(app.model.CommandsList) != 1 {
		t.Fatalf("CommandsList len = %d, want 1", len(app.model.CommandsList))
	}
	e := app.model.CommandsList[0]
	if e.Kind != ui.CommandRunKindFileExecute {
		t.Fatalf("Kind = %q, want %q", e.Kind, ui.CommandRunKindFileExecute)
	}
	if e.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0; stderr=%q err=%q", e.ExitCode, e.Stderr, e.ErrorMsg)
	}
	if !strings.Contains(e.Stdout, marker) {
		t.Fatalf("Stdout = %q, want substring %q", e.Stdout, marker)
	}
}

func TestNavOpenFalseExecutableBitDoesNotRun(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "clip.mkv")
	if err := os.WriteFile(path, []byte{0x1a, 0x45, 0xdf, 0xa3}, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.Panels.OpenFilesExternally = true
	app.config.Panels.RunExecutablesOnEnter = true
	selectEntryByName(t, app, "clip.mkv")

	app.dispatch(keymap.ActionNavOpen)

	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want ViewBrowser", app.model.ViewMode)
	}
	if len(app.model.CommandsList) != 0 {
		t.Fatalf("CommandsList len = %d, want 0 for +x non-runnable file", len(app.model.CommandsList))
	}
}

func TestNavOpenNonExecutableDoesNotRunWhenExternalOpenDisabled(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "plain.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.Panels.OpenFilesExternally = false
	app.config.Panels.RunExecutablesOnEnter = true
	selectEntryByName(t, app, "plain.txt")

	app.dispatch(keymap.ActionNavOpen)

	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want ViewBrowser", app.model.ViewMode)
	}
	if len(app.model.CommandsList) != 0 {
		t.Fatalf("CommandsList len = %d, want 0", len(app.model.CommandsList))
	}
}

func TestNavOpenSkipsExecutableWhenRunOnEnterDisabled(t *testing.T) {
	root := t.TempDir()
	writeExecutableScript(t, root, "runme.sh", "#!/bin/sh\necho hi\n")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.Panels.RunExecutablesOnEnter = false
	app.config.Panels.OpenFilesExternally = false
	selectEntryByName(t, app, "runme.sh")

	app.dispatch(keymap.ActionNavOpen)

	if len(app.model.CommandsList) != 0 {
		t.Fatalf("CommandsList len = %d, want 0 when run_executables_on_enter is false", len(app.model.CommandsList))
	}
}

func TestNavOpenExecutableUsesRelativeCommandLine(t *testing.T) {
	root := t.TempDir()
	writeExecutableScript(t, root, "runme.sh", "#!/bin/sh\necho ok\n")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	selectEntryByName(t, app, "runme.sh")

	app.dispatch(keymap.ActionNavOpen)
	waitCommandsDone(t, app)

	if len(app.model.CommandsList) != 1 {
		t.Fatal("expected one command row")
	}
	if app.model.CommandsList[0].UserCommandLine != "runme.sh" {
		t.Fatalf("UserCommandLine = %q, want runme.sh", app.model.CommandsList[0].UserCommandLine)
	}
}

func TestRunExecutablesOnEnterDefaultConfig(t *testing.T) {
	cfg := config.Default()
	if !cfg.Panels.RunExecutablesOnEnter {
		t.Fatal("Default().RunExecutablesOnEnter should be true")
	}
}
