package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/usermenu"
)

func testUserMenuApp(t *testing.T, dir, cfgDir string) *App {
	return testUserMenuAppConfig(t, dir, cfgDir, config.Default())
}

func testUserMenuAppConfig(t *testing.T, dir, cfgDir string, cfg config.Config) *App {
	t.Helper()
	cfg.UserMenu = config.UserMenuConfig{LocalNames: []string{"menu.toml"}}
	th, _ := loadTestTheme(t)
	screen := newScreen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: cfg,
		Theme:  th,
		Paths:  config.Paths{ConfigDir: cfgDir},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.model.Left.Load(dir); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestOpenUserMenuCreatesStubWhenMissing(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	app := testUserMenuApp(t, dir, cfgDir)

	var edited []string
	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		edited = append(edited, path)
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.openUserMenu()

	global := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if _, err := os.Stat(global); err != nil {
		t.Fatalf("stub not created: %v", err)
	}
	if len(edited) != 1 || edited[0] != global {
		t.Fatalf("edited = %v, want [%q]", edited, global)
	}
	if app.model.UserMenu.Open {
		t.Fatal("user menu dialog should not open during bootstrap")
	}
}

func TestEditUserMenuOpensExistingPath(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	menuBody := `[[entry]]
key = "a"
title = "Always"
command = "true"
`
	if err := os.WriteFile(menuPath, []byte(menuBody), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testUserMenuApp(t, dir, cfgDir)

	var edited []string
	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		edited = append(edited, path)
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.editUserMenu()

	if len(edited) != 1 || edited[0] != menuPath {
		t.Fatalf("edited = %v, want [%q]", edited, menuPath)
	}
	if app.model.UserMenu.Open {
		t.Fatal("edit user menu should not open dialog")
	}
}

func TestOpenUserMenuInvalidFileCritical(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(menuPath, []byte("shell_patterns = []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testUserMenuApp(t, dir, cfgDir)

	prev := externalEditorRunner
	var called bool
	externalEditorRunner = func(_ context.Context, _ string) error {
		called = true
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.openUserMenu()

	if called {
		t.Fatal("editor should not run for invalid menu.toml")
	}
	if app.model.UserMenu.Open {
		t.Fatal("user menu dialog should not open")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyCritical {
		t.Fatalf("MessageUrgency = %v, want critical", app.model.MessageUrgency)
	}
	if app.model.Message == "" {
		t.Fatal("expected critical status message")
	}
}

func TestOpenUserMenuExistingStubNoEditor(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(menuPath, []byte(usermenu.MenuStubTOML), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testUserMenuApp(t, dir, cfgDir)

	prev := externalEditorRunner
	var called bool
	externalEditorRunner = func(_ context.Context, _ string) error {
		called = true
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.openUserMenu()

	if called {
		t.Fatal("editor should not run when menu file exists but has no entries")
	}
	if app.model.UserMenu.Open {
		t.Fatal("user menu dialog should not open without entries")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyWarn {
		t.Fatalf("MessageUrgency = %v, want warn", app.model.MessageUrgency)
	}
}

func TestOpenUserMenuOpensDialogWhenEntriesExist(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(menuPath, []byte(`[[entry]]
key = "a"
title = "Always"
command = "true"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testUserMenuApp(t, dir, cfgDir)

	prev := externalEditorRunner
	var mu sync.Mutex
	called := false
	externalEditorRunner = func(_ context.Context, path string) error {
		mu.Lock()
		called = true
		mu.Unlock()
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.openUserMenu()

	if called {
		t.Fatal("editor should not run when menu has entries")
	}
	if !app.model.UserMenu.Open {
		t.Fatal("user menu dialog should open")
	}
}

func TestUserMenuEntryKeyRunsImmediately(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(menuPath, []byte(`[[entry]]
key = "a"
title = "Always"
command = "true"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testUserMenuApp(t, dir, cfgDir)
	app.openUserMenu()
	if !app.model.UserMenu.Open {
		t.Fatal("user menu should be open")
	}

	app.handleUserMenuDialogKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModAlt))

	if app.model.UserMenu.Open {
		t.Fatal("user menu should close after entry key")
	}
	if app.model.ViewMode != ui.ViewCommands {
		t.Fatalf("ViewMode = %v, want ViewCommands", app.model.ViewMode)
	}
	if len(app.model.CommandsList) != 1 {
		t.Fatalf("CommandsList len = %d, want 1", len(app.model.CommandsList))
	}
}

func TestShiftF2MapsToEditUserMenu(t *testing.T) {
	km := defaultKeymap(t)
	got := lookupActionForView(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModShift), km, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionAppUserMenuEdit {
		t.Fatalf("got %q, want %q", got, keymap.ActionAppUserMenuEdit)
	}
}

func TestWriteMenuStubMatchesPackage(t *testing.T) {
	if usermenu.MenuStubTOML == "" {
		t.Fatal("MenuStubTOML empty")
	}
}

func writeUserMenuFile(t *testing.T, menuPath, body string) {
	t.Helper()
	if err := os.WriteFile(menuPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writePoolsFile(t *testing.T, poolsPath, body string) {
	t.Helper()
	if err := os.WriteFile(poolsPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUserMenuInteractiveDoesNotOpenCommandsView(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeUserMenuFile(t, menuPath, `[[entry]]
key = "g"
title = "lazygit"
command = "lazygit"
interactive = true
`)

	app := testUserMenuApp(t, dir, cfgDir)

	var gotArgv []string
	var gotDir string
	prev := userMenuInteractiveRunner
	userMenuInteractiveRunner = func(_ context.Context, argv []string, dir string) error {
		gotArgv = append([]string(nil), argv...)
		gotDir = dir
		return nil
	}
	t.Cleanup(func() { userMenuInteractiveRunner = prev })

	app.openUserMenu()
	app.handleUserMenuDialogKey(tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModAlt))

	if app.model.ViewMode == ui.ViewCommands {
		t.Fatal("interactive user menu should not open commands view")
	}
	if len(app.model.CommandsList) != 0 {
		t.Fatalf("CommandsList len = %d, want 0", len(app.model.CommandsList))
	}
	if len(gotArgv) != 1 || gotArgv[0] != "lazygit" {
		t.Fatalf("argv = %v, want [lazygit]", gotArgv)
	}
	if gotDir != dir {
		t.Fatalf("workDir = %q, want %q", gotDir, dir)
	}
}

func TestUserMenuDetachDoesNotOpenCommandsView(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeUserMenuFile(t, menuPath, `[[entry]]
key = "o"
title = "Open"
command = "xdg-open ."
detach = true
`)

	app := testUserMenuApp(t, dir, cfgDir)

	var gotArgv []string
	var gotDir string
	wantDir := dir
	prev := userMenuDetachRunner
	userMenuDetachRunner = func(argv []string, workDir string) error {
		gotArgv = append([]string(nil), argv...)
		gotDir = workDir
		return nil
	}
	t.Cleanup(func() { userMenuDetachRunner = prev })

	app.openUserMenu()
	app.handleUserMenuDialogKey(tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModAlt))

	if app.model.ViewMode == ui.ViewCommands {
		t.Fatal("detach user menu should not open commands view")
	}
	if len(app.model.CommandsList) != 0 {
		t.Fatalf("CommandsList len = %d, want 0", len(app.model.CommandsList))
	}
	if len(gotArgv) != 2 || gotArgv[0] != "xdg-open" || gotArgv[1] != "." {
		t.Fatalf("argv = %v", gotArgv)
	}
	if gotDir != wantDir {
		t.Fatalf("workDir = %q, want %q", gotDir, wantDir)
	}
}

func TestUserMenuBackgroundDoesNotOpenCommandsView(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeUserMenuFile(t, menuPath, `[[entry]]
key = "a"
title = "Always"
command = "true"
background = true
`)

	app := testUserMenuApp(t, dir, cfgDir)
	app.openUserMenu()
	app.handleUserMenuDialogKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModAlt))

	if app.model.ViewMode == ui.ViewCommands {
		t.Fatal("background user menu should not open commands view")
	}
	if len(app.model.CommandsList) != 1 {
		t.Fatalf("CommandsList len = %d, want 1", len(app.model.CommandsList))
	}
	waitCommandsDone(t, app)
}

func TestUserMenuBackgroundNotifiesOnFailure(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeUserMenuFile(t, menuPath, `[[entry]]
key = "f"
title = "Fail"
command = "false"
background = true
`)

	app := testUserMenuApp(t, dir, cfgDir)
	app.openUserMenu()
	app.handleUserMenuDialogKey(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt))
	waitCommandsDone(t, app)

	app.applyCommandWake(backgroundWakePayloadForEntry(t, app, "Fail"))
	if app.model.MessageUrgency != ui.MessageUrgencyError {
		t.Fatalf("MessageUrgency = %v, want error", app.model.MessageUrgency)
	}
	if !strings.Contains(app.model.Message, "Fail") {
		t.Fatalf("Message = %q, want title in banner", app.model.Message)
	}
}

func TestUserMenuBackgroundRefreshesPanelOnCompletion(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	marker := "paras_bg_refresh_marker"
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeUserMenuFile(t, menuPath, `[[entry]]
key = "t"
title = "Touch"
command = "touch `+marker+`"
background = true
`)

	app := testUserMenuApp(t, dir, cfgDir)
	app.openUserMenu()
	app.handleUserMenuDialogKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModAlt))
	waitCommandsDone(t, app)

	app.applyCommandWake(commandWakePayload{refreshBrowserPanel: true})
	selectEntryByName(t, app, marker)
}

func TestUserMenuBackgroundNotify(t *testing.T) {
	log, banner, urg, ok := userMenuBackgroundNotify("Build", cmdrun.RunResult{LaunchErr: errors.New("executable not found")})
	if !ok || urg != ui.MessageUrgencyError || !strings.Contains(log, "Build") || banner == "" {
		t.Fatalf("launch err: ok=%v urg=%v log=%q banner=%q", ok, urg, log, banner)
	}

	log, _, urg, ok = userMenuBackgroundNotify("Lint", cmdrun.RunResult{ExitCode: 2, Stderr: []byte("syntax error\n")})
	if !ok || urg != ui.MessageUrgencyError || !strings.Contains(log, "exit 2") {
		t.Fatalf("exit+stderr: ok=%v urg=%v log=%q", ok, urg, log)
	}

	log, _, urg, ok = userMenuBackgroundNotify("Warn", cmdrun.RunResult{ExitCode: 0, Stderr: []byte("note\n")})
	if !ok || urg != ui.MessageUrgencyWarn {
		t.Fatalf("stderr only: ok=%v urg=%v log=%q", ok, urg, log)
	}

	_, _, _, ok = userMenuBackgroundNotify("OK", cmdrun.RunResult{ExitCode: 0})
	if ok {
		t.Fatal("clean success should not notify")
	}
}

func TestUserMenuWorkPoolLimitsParallelism(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeUserMenuFile(t, menuPath, `[[entry]]
key = "s"
title = "Sleep"
command = "sleep 0.4"
pool = "slow"
background = true
`)
	writePoolsFile(t, filepath.Join(cfgDir, config.DefaultPoolsFileName), `[[pools]]
name = "slow"
max_parallel = 1
`)

	app := testUserMenuApp(t, dir, cfgDir)

	app.openUserMenu()
	app.handleUserMenuDialogKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModAlt))
	app.openUserMenu()
	app.handleUserMenuDialogKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModAlt))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		app.commandsMu.RLock()
		running, pending := 0, 0
		for _, e := range app.model.CommandsList {
			switch e.Phase {
			case ui.CommandRunRunning:
				running++
			case ui.CommandRunPending:
				pending++
			}
		}
		app.commandsMu.RUnlock()
		if running == 1 && pending == 1 {
			waitCommandsDone(t, app)
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("expected one Running and one Pending while pool max_parallel=1")
}

func TestUserMenuUnknownWorkPool(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeUserMenuFile(t, menuPath, `[[entry]]
key = "x"
title = "Bad pool"
command = "true"
pool = "missing"
`)

	app := testUserMenuApp(t, dir, cfgDir)
	app.openUserMenu()
	app.handleUserMenuDialogKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModAlt))
	waitCommandsDone(t, app)

	app.commandsMu.RLock()
	defer app.commandsMu.RUnlock()
	if len(app.model.CommandsList) != 1 {
		t.Fatalf("CommandsList len = %d, want 1", len(app.model.CommandsList))
	}
	e := app.model.CommandsList[0]
	if !strings.Contains(e.ErrorMsg, "unknown work pool") {
		t.Fatalf("ErrorMsg = %q, want unknown work pool", e.ErrorMsg)
	}
}

func backgroundWakePayloadForEntry(t *testing.T, app *App, title string) commandWakePayload {
	t.Helper()
	if len(app.model.CommandsList) == 0 {
		t.Fatal("no command rows")
	}
	e := app.model.CommandsList[len(app.model.CommandsList)-1]
	res := cmdrun.RunResult{
		Stdout:   []byte(e.Stdout),
		Stderr:   []byte(e.Stderr),
		ExitCode: e.ExitCode,
	}
	if e.ErrorMsg != "" {
		res.LaunchErr = errors.New(e.ErrorMsg)
	}
	p := commandWakePayload{refreshBrowserPanel: true}
	if log, banner, urg, ok := userMenuBackgroundNotify(title, res); ok {
		p.notifyLog = log
		p.notifyBanner = banner
		p.notifyUrg = urg
	}
	return p
}
