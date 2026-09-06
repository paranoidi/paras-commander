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
	commandsctrl "github.com/paranoidi/paras-commander/internal/apphandler/commands"
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
	app := newTestApp(t, screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: cfg,
		Theme:  th,
		Paths:  config.Paths{ConfigDir: cfgDir},
	})
	if err := app.model.Primary.Load(dir); err != nil {
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
	if app.model.LeaderMenu.Open {
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
	menuBody := `[always]
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
	if app.model.LeaderMenu.Open {
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
	if app.model.LeaderMenu.Open {
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
	if app.model.LeaderMenu.Open {
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
	if err := os.WriteFile(menuPath, []byte(`[always]
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
	if !app.model.LeaderMenu.Open {
		t.Fatal("user menu dialog should open")
	}
	for _, it := range app.model.LeaderMenu.Items {
		if it.DirectKey != "" {
			t.Fatalf("user menu item %q has DirectKey %q, want empty", it.Label, it.DirectKey)
		}
	}
}

func TestF2KeyTogglesUserMenuClosed(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(menuPath, []byte(`[always]
key = "a"
title = "Always"
command = "true"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testUserMenuApp(t, dir, cfgDir)
	app.model.ViewMode = ui.ViewBrowser

	app.toggleUserMenu()
	if !app.userMenuOpen() {
		t.Fatal("expected user menu open")
	}

	f2 := tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone)
	_, rendered := app.handleKey(f2)
	if !rendered {
		t.Fatal("second F2 should render after closing the user menu")
	}
	if app.model.LeaderMenu.Open {
		t.Fatal("second F2 should close the user menu")
	}
	if len(app.userMenuStack) != 0 {
		t.Fatalf("userMenuStack len = %d, want 0 after toggle close", len(app.userMenuStack))
	}
}

func TestUserMenuEntryKeyRunsImmediately(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(menuPath, []byte(`[always]
key = "a"
title = "Always"
command = "true"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testUserMenuApp(t, dir, cfgDir)
	app.openUserMenu()
	if !app.model.LeaderMenu.Open {
		t.Fatal("user menu should be open")
	}

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone))

	if app.model.LeaderMenu.Open {
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
	got := lookupActionForView(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModShift), km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
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
	writeUserMenuFile(t, menuPath, `[lazygit]
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
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone))

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
	writeUserMenuFile(t, menuPath, `[open]
key = "p"
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
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModNone))

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
	writeUserMenuFile(t, menuPath, `[always]
key = "a"
title = "Always"
command = "true"
background = true
`)

	app := testUserMenuApp(t, dir, cfgDir)
	app.openUserMenu()
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone))

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
	writeUserMenuFile(t, menuPath, `[fail]
key = "f"
title = "Fail"
command = "false"
background = true
`)

	app := testUserMenuApp(t, dir, cfgDir)
	app.openUserMenu()
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModNone))
	waitCommandsDone(t, app)

	app.commandsCtrl.ApplyWake(backgroundWakePayloadForEntry(t, app, "Fail"))
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
	writeUserMenuFile(t, menuPath, `[touch]
key = "t"
title = "Touch"
command = "touch `+marker+`"
background = true
`)

	app := testUserMenuApp(t, dir, cfgDir)
	app.openUserMenu()
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))
	waitCommandsDone(t, app)

	app.commandsCtrl.ApplyWake(commandsctrl.WakePayload{RefreshBrowserPanel: true})
	// RefreshAfterUserMenuCommand's reload is async; drain until the newly-created marker file
	// actually shows up rather than assuming the very next screen event is that reload landing.
	screen := app.screen.(tcell.SimulationScreen)
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool {
		p := app.activePanel()
		for i := 0; i < p.VisibleEntryCount(); i++ {
			if e, _, ok := p.VisibleEntry(i); ok && e.Name == marker {
				return true
			}
		}
		return false
	})
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
	writeUserMenuFile(t, menuPath, `[sleep]
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
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone))
	app.openUserMenu()
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone))

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
	writeUserMenuFile(t, menuPath, `[bad_pool]
key = "x"
title = "Bad pool"
command = "true"
pool = "missing"
`)

	app := testUserMenuApp(t, dir, cfgDir)
	app.openUserMenu()

	if app.model.LeaderMenu.Open {
		t.Fatal("user menu dialog should not open for unknown pool")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyCritical {
		t.Fatalf("MessageUrgency = %v, want critical", app.model.MessageUrgency)
	}
	if !strings.Contains(app.model.Message, "unknown pool") {
		t.Fatalf("Message = %q, want unknown pool", app.model.Message)
	}
}

func userMenuSubmenuFixture(t *testing.T, menuPath string) {
	t.Helper()
	writeUserMenuFile(t, menuPath, `[tools]
key = "t"
title = "Tools"

[tools.disk_use]
key = "d"
title = "Show disk usage"
command = "true"

[always]
key = "a"
title = "Always"
command = "true"
`)
}

func TestUserMenuSubmenuOpensChildren(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userMenuSubmenuFixture(t, menuPath)

	app := testUserMenuApp(t, dir, cfgDir)
	app.openUserMenu()
	if len(app.model.LeaderMenu.Items) != 2 {
		t.Fatalf("top-level items len = %d, want 2", len(app.model.LeaderMenu.Items))
	}

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))

	if !app.model.LeaderMenu.Open {
		t.Fatal("leader menu should stay open after entering submenu")
	}
	if len(app.userMenuVisible) != 1 || app.userMenuVisible[0].Title != "Show disk usage" {
		t.Fatalf("userMenuVisible = %+v, want submenu children", app.userMenuVisible)
	}
	if len(app.model.LeaderMenu.Items) != 1 || app.model.LeaderMenu.Items[0].Label != "Show disk usage" {
		t.Fatalf("items = %+v, want submenu children", app.model.LeaderMenu.Items)
	}
	if len(app.userMenuStack) != 1 {
		t.Fatalf("userMenuStack len = %d, want 1 after entering submenu", len(app.userMenuStack))
	}
}

func TestUserMenuEscInSubmenuReturnsToParentLevel(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userMenuSubmenuFixture(t, menuPath)

	app := testUserMenuApp(t, dir, cfgDir)
	app.openUserMenu()
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))
	if len(app.model.LeaderMenu.Items) != 1 {
		t.Fatalf("test setup: expected to be inside submenu, items = %+v", app.model.LeaderMenu.Items)
	}

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))

	if !app.model.LeaderMenu.Open {
		t.Fatal("Esc inside submenu should return to parent level, not close the whole menu")
	}
	if len(app.model.LeaderMenu.Items) != 2 {
		t.Fatalf("items after Esc = %+v, want back to top-level 2 items", app.model.LeaderMenu.Items)
	}
	if len(app.userMenuStack) != 0 {
		t.Fatalf("userMenuStack len = %d, want 0 back at top level", len(app.userMenuStack))
	}
}

func TestUserMenuEscAtTopLevelClosesMenu(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userMenuSubmenuFixture(t, menuPath)

	app := testUserMenuApp(t, dir, cfgDir)
	app.openUserMenu()

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))

	if app.model.LeaderMenu.Open {
		t.Fatal("Esc at top level should close the whole menu")
	}
}

func TestUserMenuLeafRunFromSubmenuClearsStack(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userMenuSubmenuFixture(t, menuPath)

	app := testUserMenuApp(t, dir, cfgDir)
	app.openUserMenu()
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))

	if app.model.LeaderMenu.Open {
		t.Fatal("leader menu should close after running a leaf from inside a submenu")
	}
	if len(app.userMenuStack) != 0 {
		t.Fatalf("userMenuStack len = %d, want 0 after running a leaf", len(app.userMenuStack))
	}

	app.model.ViewMode = ui.ViewBrowser
	app.openUserMenu()
	if len(app.model.LeaderMenu.Items) != 2 {
		t.Fatalf("next F2 open items = %+v, want fresh top-level 2 items", app.model.LeaderMenu.Items)
	}
}

func backgroundWakePayloadForEntry(t *testing.T, app *App, title string) commandsctrl.WakePayload {
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
	p := commandsctrl.WakePayload{RefreshBrowserPanel: true}
	if log, banner, urg, ok := userMenuBackgroundNotify(title, res); ok {
		p.NotifyLog = log
		p.NotifyBanner = banner
		p.NotifyUrg = urg
	}
	return p
}
