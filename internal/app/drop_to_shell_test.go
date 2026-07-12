package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestDropToShellBlocksRemotePanel(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.model.Primary.Path = pathloc.MustParse("sftp://user@example.com/")

	var calls int
	prev := dropToShellRunner
	dropToShellRunner = func(_ context.Context, _ []string) error {
		calls++
		return nil
	}
	t.Cleanup(func() { dropToShellRunner = prev })

	app.dropToShell()
	if calls != 0 {
		t.Fatalf("dropToShellRunner calls = %d, want 0 for remote panel", calls)
	}
}

func TestDropToShellStartsInPanelDirectory(t *testing.T) {
	root := t.TempDir()
	panelDir := filepath.Join(root, "alpha")
	if err := os.Mkdir(panelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	if err := app.model.Primary.Load(panelDir); err != nil {
		t.Fatal(err)
	}

	var gotWd string
	prev := dropToShellRunner
	dropToShellRunner = func(_ context.Context, _ []string) error {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd in shell: %v", err)
		}
		gotWd = wd
		return nil
	}
	t.Cleanup(func() { dropToShellRunner = prev })

	app.dropToShell()
	if gotWd != panelDir {
		t.Fatalf("shell cwd = %q, want %q", gotWd, panelDir)
	}
}

func TestDropToShellSyncsPanelCwdOnReturn(t *testing.T) {
	root := t.TempDir()
	panelDir := filepath.Join(root, "alpha")
	otherDir := filepath.Join(root, "beta")
	for _, d := range []string{panelDir, otherDir} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	if err := app.model.Primary.Load(panelDir); err != nil {
		t.Fatal(err)
	}

	prev := dropToShellRunner
	dropToShellRunner = func(_ context.Context, _ []string) error {
		return os.Chdir(otherDir)
	}
	t.Cleanup(func() { dropToShellRunner = prev })

	app.dropToShell()
	if got := filepath.Clean(app.model.Primary.PathString()); got != otherDir {
		t.Fatalf("panel path = %q, want %q", got, otherDir)
	}
}

func TestDropToShellSkipsSyncWhenDisabled(t *testing.T) {
	root := t.TempDir()
	panelDir := filepath.Join(root, "alpha")
	otherDir := filepath.Join(root, "beta")
	for _, d := range []string{panelDir, otherDir} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.Shell.SyncCwdOnReturn = false
	if err := app.model.Primary.Load(panelDir); err != nil {
		t.Fatal(err)
	}

	prev := dropToShellRunner
	dropToShellRunner = func(_ context.Context, _ []string) error {
		return os.Chdir(otherDir)
	}
	t.Cleanup(func() { dropToShellRunner = prev })

	app.dropToShell()
	if got := filepath.Clean(app.model.Primary.PathString()); got != panelDir {
		t.Fatalf("panel path = %q, want unchanged %q", got, panelDir)
	}
}

func TestDropToShellUsesConfigCommandOverride(t *testing.T) {
	root := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	if err := app.model.Primary.Load(root); err != nil {
		t.Fatal(err)
	}
	app.config.Shell.Command = "/bin/echo drop-shell-marker"

	var gotArgv []string
	prev := dropToShellRunner
	dropToShellRunner = func(_ context.Context, argv []string) error {
		gotArgv = append([]string(nil), argv...)
		return nil
	}
	t.Cleanup(func() { dropToShellRunner = prev })

	app.dropToShell()
	if len(gotArgv) != 2 || gotArgv[0] != "/bin/echo" || gotArgv[1] != "drop-shell-marker" {
		t.Fatalf("argv = %v, want [/bin/echo drop-shell-marker]", gotArgv)
	}
}

func TestShellArgvDefaultUsesResolveShell(t *testing.T) {
	app := testAppMinimal(t)
	app.config.Shell.Command = ""
	argv, err := app.shellArgv()
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) != 1 || argv[0] == "" {
		t.Fatalf("shellArgv() = %v, want single shell path", argv)
	}
}

func TestDispatchDropToShellAction(t *testing.T) {
	root := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	if err := app.model.Primary.Load(root); err != nil {
		t.Fatal(err)
	}

	var called bool
	prev := dropToShellRunner
	dropToShellRunner = func(_ context.Context, _ []string) error {
		called = true
		return nil
	}
	t.Cleanup(func() { dropToShellRunner = prev })

	app.dispatch(keymap.ActionAppDropToShell)
	if !called {
		t.Fatal("dispatch app.drop-to-shell did not invoke shell runner")
	}
}

func TestDropToShellDefaultConfigSyncEnabled(t *testing.T) {
	cfg := config.Default()
	if !cfg.Shell.SyncCwdOnReturn {
		t.Fatal("Default().Shell.SyncCwdOnReturn = false, want true")
	}
	if !cfg.Shell.Persistent {
		t.Fatal("Default().Shell.Persistent = false, want true")
	}
}

func TestShellInsertPathListSortsSelections(t *testing.T) {
	p := &panel.State{SelectedPaths: map[string]bool{
		"/tmp/orchard/walnut": true,
		"/tmp/meadow/aspen":   true,
		"/tmp/meadow/birch":   true,
	}}
	got := shellInsertPathList(p)
	want := []string{"/tmp/meadow/aspen", "/tmp/meadow/birch", "/tmp/orchard/walnut"}
	if len(got) != len(want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("paths = %v, want %v", got, want)
		}
	}
}

func TestAltEnterResolvesToShellInsertPaths(t *testing.T) {
	app := testAppMinimal(t)
	for _, mod := range []tcell.ModMask{tcell.ModAlt, tcell.ModShift} {
		id, ok := app.keys.Lookup(tcell.NewEventKey(tcell.KeyEnter, '\r', mod))
		if !ok || id != keymap.ActionAppShellInsertPaths {
			t.Fatalf("Enter+%v resolves to %q (ok=%v), want %q", mod, id, ok, keymap.ActionAppShellInsertPaths)
		}
	}
}

func TestDispatchShellInsertPathsWithoutPersistentWarns(t *testing.T) {
	root := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root) // helper disables Shell.Persistent
	if err := app.model.Primary.Load(root); err != nil {
		t.Fatal(err)
	}

	app.dispatch(keymap.ActionAppShellInsertPaths)
	if app.subshell != nil {
		t.Fatal("no subshell session should be started without persistent shell")
	}
}

func TestPersistentShellSkippedWithCommandOverride(t *testing.T) {
	root := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	if err := app.model.Primary.Load(root); err != nil {
		t.Fatal(err)
	}
	app.config.Shell.Command = "/bin/echo drop-shell-marker"

	if app.persistentShellToggle() {
		t.Fatal("persistentShellToggle should refuse a custom shell.command")
	}
	if app.subshell != nil {
		t.Fatal("no subshell session should be started")
	}
}
