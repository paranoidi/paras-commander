package app

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	commandsctrl "github.com/paranoidi/paras-commander/internal/apphandler/commands"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
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

// stubDetach replaces the detached-process runner for the test; a nil fn accepts every launch.
func stubDetach(t *testing.T, fn func(argv []string, workDir string) error) {
	t.Helper()
	if fn == nil {
		fn = func([]string, string) error { return nil }
	}
	prev := userMenuDetachRunner
	userMenuDetachRunner = fn
	t.Cleanup(func() { userMenuDetachRunner = prev })
}

func TestOpenUserMenuCreatesStubWhenMissing(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	app := testUserMenuApp(t, dir, cfgDir)

	var edited []string
	app.runExternalEditor = func(_ context.Context, path string, _ bool) error {
		edited = append(edited, path)
		return nil
	}

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
	app.runExternalEditor = func(_ context.Context, path string, _ bool) error {
		edited = append(edited, path)
		return nil
	}

	app.editUserMenu()

	if len(edited) != 1 || edited[0] != menuPath {
		t.Fatalf("edited = %v, want [%q]", edited, menuPath)
	}
	if app.model.LeaderMenu.Open {
		t.Fatal("edit user menu should not open dialog")
	}
}

// assertUserMenuDisabled checks a broken menu.toml still opens the user-menu strip with a
// the error as key-less title rows, so the edit-config key stays reachable.
func assertUserMenuDisabled(t *testing.T, app *App) {
	t.Helper()
	st := app.model.LeaderMenu
	if !st.Open || st.Kind != ui.LeaderMenuUser {
		t.Fatalf("LeaderMenu = %+v, want open user menu", st)
	}
	if len(st.Items) == 0 {
		t.Fatal("want the error as title rows")
	}
	for _, it := range st.Items {
		if it.GroupTitle == "" {
			t.Fatalf("items = %+v, want only title rows", st.Items)
		}
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

	var called bool
	app.runExternalEditor = func(_ context.Context, _ string, _ bool) error {
		called = true
		return nil
	}

	app.openUserMenu()

	if called {
		t.Fatal("editor should not run for invalid menu.toml")
	}
	assertUserMenuDisabled(t, app)
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

	var called bool
	app.runExternalEditor = func(_ context.Context, _ string, _ bool) error {
		called = true
		return nil
	}

	app.openUserMenu()

	if called {
		t.Fatal("editor should not run when menu file exists but has no entries")
	}
	assertUserMenuDisabled(t, app)
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

	var mu sync.Mutex
	called := false
	app.runExternalEditor = func(_ context.Context, path string, _ bool) error {
		mu.Lock()
		called = true
		mu.Unlock()
		return nil
	}

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

func TestOpenUserMenuWarnsOnDuplicateKey(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(menuPath, []byte(`[one]
key = "a"
title = "One"
command = "true"

[two]
key = "a"
title = "Two"
command = "true"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testUserMenuApp(t, dir, cfgDir)

	app.openUserMenu()

	if !app.model.LeaderMenu.Open {
		t.Fatal("user menu dialog should open despite the duplicate key")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyWarn {
		t.Fatalf("MessageUrgency = %v, want warn", app.model.MessageUrgency)
	}
	if !strings.Contains(app.model.Message, "duplicate key") {
		t.Fatalf("Message = %q, want it to mention duplicate key", app.model.Message)
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
	if !app.leaderMenuOpen(ui.LeaderMenuUser) {
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
	stubDetach(t, func(argv []string, workDir string) error {
		gotArgv = append([]string(nil), argv...)
		gotDir = workDir
		return nil
	})

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
	// RefreshAfterBackgroundCommand's reload is async; drain until the newly-created marker file
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
	log, banner, urg, ok := backgroundRunNotify("Build", cmdrun.RunResult{LaunchErr: errors.New("executable not found")})
	if !ok || urg != ui.MessageUrgencyError || !strings.Contains(log, "Build") || banner == "" {
		t.Fatalf("launch err: ok=%v urg=%v log=%q banner=%q", ok, urg, log, banner)
	}

	log, _, urg, ok = backgroundRunNotify("Lint", cmdrun.RunResult{ExitCode: 2, Stderr: []byte("syntax error\n")})
	if !ok || urg != ui.MessageUrgencyError || !strings.Contains(log, "exit 2") {
		t.Fatalf("exit+stderr: ok=%v urg=%v log=%q", ok, urg, log)
	}

	log, _, urg, ok = backgroundRunNotify("Warn", cmdrun.RunResult{ExitCode: 0, Stderr: []byte("note\n")})
	if !ok || urg != ui.MessageUrgencyWarn {
		t.Fatalf("stderr only: ok=%v urg=%v log=%q", ok, urg, log)
	}

	_, _, _, ok = backgroundRunNotify("OK", cmdrun.RunResult{ExitCode: 0})
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

	assertUserMenuDisabled(t, app)
	if app.model.MessageUrgency != ui.MessageUrgencyCritical {
		t.Fatalf("MessageUrgency = %v, want critical", app.model.MessageUrgency)
	}
	if !strings.Contains(app.model.Message, "unknown pool") || !strings.Contains(app.model.LeaderMenu.Items[0].GroupTitle, "unknown pool") {
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

func TestUserMenuBackInSubmenuReturnsToParentLevel(t *testing.T) {
	for _, ev := range []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone),
	} {
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

		app.handleLeaderMenuKey(ev)

		if !app.model.LeaderMenu.Open {
			t.Fatalf("%v inside submenu should return to parent level, not close the whole menu", ev.Name())
		}
		if len(app.model.LeaderMenu.Items) != 2 {
			t.Fatalf("items after %v = %+v, want back to top-level 2 items", ev.Name(), app.model.LeaderMenu.Items)
		}
		if len(app.userMenuStack) != 0 {
			t.Fatalf("userMenuStack len = %d, want 0 back at top level", len(app.userMenuStack))
		}

		// Back at top level closes the menu.
		app.handleLeaderMenuKey(ev)
		if app.model.LeaderMenu.Open {
			t.Fatalf("%v at top level should close the menu", ev.Name())
		}
	}
}

func TestUserMenuEscInSubmenuClosesWholeMenu(t *testing.T) {
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

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))

	if app.model.LeaderMenu.Open {
		t.Fatal("Esc inside submenu should close the whole menu")
	}
	if len(app.userMenuStack) != 0 {
		t.Fatalf("userMenuStack len = %d, want 0 after close", len(app.userMenuStack))
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
	if log, banner, urg, ok := backgroundRunNotify("User menu: "+backgroundRunTitle(title), res); ok {
		p.NotifyLog = log
		p.NotifyBanner = banner
		p.NotifyUrg = urg
	}
	return p
}

func TestUserMenuKeepOpenStaysOpenWithToast(t *testing.T) {
	for _, tc := range []struct {
		toast string
		nerd  bool
		want  string
	}{{"", false, "Ran Quartz"}, {"", true, "\uf05d Quartz"}, {"Custom note", true, "Custom note"}} {
		dir := t.TempDir()
		cfgDir := filepath.Join(dir, "config")
		menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
		if err := os.MkdirAll(cfgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		toastLine := ""
		if tc.toast != "" {
			toastLine = "toast = \"" + tc.toast + "\"\n"
		}
		writeUserMenuFile(t, menuPath, "[q]\nkey = \"q\"\ntitle = \"Quartz\"\ncommand = \"true\"\ndetach = true\nkeep_open = true\n"+toastLine+
			"\n[r]\nkey = \"r\"\ntitle = \"Ridge\"\ncommand = \"true\"\ndetach = true\n")
		app := testUserMenuApp(t, dir, cfgDir)
		app.styles.UseNerdfontIcons = tc.nerd
		stubDetach(t, nil)

		app.openUserMenu()
		app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone))
		if !app.leaderMenuOpen(ui.LeaderMenuUser) {
			t.Fatal("menu should stay open")
		}
		if app.model.Message != tc.want {
			t.Fatalf("Message = %q, want %q", app.model.Message, tc.want)
		}
		app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone))
		if app.leaderMenuOpen(ui.LeaderMenuUser) {
			t.Fatal("non-keep_open entry should close the menu")
		}
	}
}

func TestUserMenuViHClosesTopLevel(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userMenuSubmenuFixture(t, menuPath)

	app := testUserMenuApp(t, dir, cfgDir)
	app.model.ViMotionMode = true
	app.openUserMenu()
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModNone))
	if app.model.LeaderMenu.Open {
		t.Fatal("h in vi-motion mode at top level should close the user menu")
	}
}

func TestUserMenuClearsOnlyConsumedSelection(t *testing.T) {
	cases := []struct {
		name, cmd             string
		wantActive, wantOther int
	}{
		{"tagged in active", "echo %t", 0, 2},
		{"tagged in other", "echo %T", 2, 0},
		{"directory only", "echo %d", 2, 2},
		{"fallback clears the tags it used", "echo %{t|d}", 0, 2},
		{"fallback that never reaches tags", "echo %{d|t}", 2, 2},
		{"fallback does not cascade across panels", "echo %{t|T}", 0, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgDir := filepath.Join(dir, "config")
			if err := os.MkdirAll(cfgDir, 0o755); err != nil {
				t.Fatal(err)
			}
			writeUserMenuFile(t, filepath.Join(cfgDir, config.DefaultUserMenuFileName),
				"[run]\nkey = \"r\"\ntitle = \"Run\"\ncommand = \""+tc.cmd+"\"\ndetach = true\n")
			app := testUserMenuApp(t, dir, cfgDir)
			tag := func(p *panel.State, root string) {
				p.SelectedPaths = map[string]bool{
					filepath.Join(root, "harbor.txt"): true,
					filepath.Join(root, "walnut.txt"): true,
				}
			}
			tag(&app.model.Primary, dir)
			tag(&app.model.Secondary, app.model.Secondary.PathString())

			stubDetach(t, nil)

			app.openUserMenu()
			app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone))

			if got := len(app.model.Primary.SelectedPaths); got != tc.wantActive {
				t.Fatalf("active tags = %d, want %d", got, tc.wantActive)
			}
			if got := len(app.model.Secondary.SelectedPaths); got != tc.wantOther {
				t.Fatalf("other tags = %d, want %d", got, tc.wantOther)
			}
		})
	}
}

// userMenuTreeFixture builds root/{orchard/ember.txt, lantern.txt}, shows the primary panel in
// tree layout with orchard expanded and the user menu file at cfgDir (outside root).
func userMenuTreeFixture(t *testing.T, menuBody string) (app *App, root, sub string) {
	t.Helper()
	return userMenuTreeFixtureWith(t, menuBody, nil)
}

// userMenuTreeFixtureWith is userMenuTreeFixture with a browse-with index overlay on the panel
// (nil for none), which adds ghost rows to the expanded directories.
func userMenuTreeFixtureWith(t *testing.T, menuBody string, overlay panel.IndexOverlayFunc) (app *App, root, sub string) {
	t.Helper()
	root = t.TempDir()
	sub = filepath.Join(root, "orchard")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{filepath.Join(sub, "ember.txt"), filepath.Join(root, "lantern.txt")} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfgDir := t.TempDir()
	writeUserMenuFile(t, filepath.Join(cfgDir, config.DefaultUserMenuFileName), menuBody)
	app = testUserMenuApp(t, root, cfgDir)
	// Without a scheduler the expansion loads synchronously, so no async wake is awaited.
	app.model.Primary.ScheduleTreeChildLoad = nil
	want := 3
	if overlay != nil {
		app.model.Primary.WithIndex = true
		app.model.Primary.IndexOverlay = overlay
		want = 4
	}
	// Directories sort first, so the cursor sits on orchard and the toggle expands it.
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleTree)
	if got := app.model.Primary.VisibleEntryCount(); got != want {
		t.Fatalf("VisibleEntryCount = %d, want %d", got, want)
	}
	return app, root, sub
}

func runUserMenuKey(app *App, key rune) {
	app.openUserMenu()
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, key, tcell.ModNone))
}

func assertFileExists(t *testing.T, path string, want bool) {
	t.Helper()
	_, err := os.Stat(path)
	if (err == nil) != want {
		t.Fatalf("exists(%s) = %v, want %v", path, err == nil, want)
	}
}

func TestUserMenuTreeCursorRowRunsInItsDirectory(t *testing.T) {
	app, root, sub := userMenuTreeFixture(t, "[touch]\nkey = \"t\"\ntitle = \"Touch\"\ncommand = \"touch foobar\"\n")
	selectEntryByName(t, app, "ember.txt")

	runUserMenuKey(app, 't')
	waitCommandsDone(t, app)

	assertFileExists(t, filepath.Join(sub, "foobar"), true)
	assertFileExists(t, filepath.Join(root, "foobar"), false)
}

func TestUserMenuTreeInteractiveGetsCursorRowDirectory(t *testing.T) {
	cases := []struct {
		name, row string
		want      func(root, sub string) string
	}{
		{"child row", "ember.txt", func(_, sub string) string { return sub }},
		{"directory row is held by the root", "orchard", func(root, _ string) string { return root }},
		{"root-level row", "lantern.txt", func(root, _ string) string { return root }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, root, sub := userMenuTreeFixture(t, "[run]\nkey = \"g\"\ntitle = \"Run\"\ncommand = \"lazygit\"\ninteractive = true\n")
			var gotDir string
			prev := userMenuInteractiveRunner
			userMenuInteractiveRunner = func(_ context.Context, _ []string, dir string) error {
				gotDir = dir
				return nil
			}
			t.Cleanup(func() { userMenuInteractiveRunner = prev })
			selectEntryByName(t, app, tc.row)

			runUserMenuKey(app, 'g')

			if want := tc.want(root, sub); gotDir != want {
				t.Fatalf("dir = %q, want %q", gotDir, want)
			}
		})
	}
}

func TestUserMenuTreeRootRowRunsInPanelRoot(t *testing.T) {
	app, root, sub := userMenuTreeFixture(t, "[touch]\nkey = \"t\"\ntitle = \"Touch\"\ncommand = \"touch foobar\"\n")
	selectEntryByName(t, app, "lantern.txt")

	runUserMenuKey(app, 't')
	waitCommandsDone(t, app)

	assertFileExists(t, filepath.Join(root, "foobar"), true)
	assertFileExists(t, filepath.Join(sub, "foobar"), false)
}

func TestUserMenuRunForEachTreeItemsRunInOwnDirectory(t *testing.T) {
	app, root, sub := userMenuTreeFixture(t, "[each]\nkey = \"e\"\ntitle = \"Each\"\ncommand = \"touch stamp && echo %f\"\nrun_for_each = [\"files\"]\n")
	p := &app.model.Primary
	p.SelectedPaths = map[string]bool{
		filepath.Join(root, "lantern.txt"): true,
		filepath.Join(sub, "ember.txt"):    true,
	}

	runUserMenuKey(app, 'e')
	waitCommandsDone(t, app)

	assertFileExists(t, filepath.Join(root, "stamp"), true)
	assertFileExists(t, filepath.Join(sub, "stamp"), true)
}

func TestRunForEachDialogTreeItemsRunInOwnDirectory(t *testing.T) {
	app, root, sub := userMenuTreeFixture(t, "")
	app.model.Primary.SelectedPaths = map[string]bool{
		filepath.Join(root, "lantern.txt"): true,
		filepath.Join(sub, "ember.txt"):    true,
	}

	app.commandsCtrl.OpenRunForEachDialog()
	app.model.FileDialog.Fields[0].Value = "touch stamp && echo %f"
	app.commandsCtrl.ExecuteRunForEach()
	waitCommandsDone(t, app)

	assertFileExists(t, filepath.Join(root, "stamp"), true)
	assertFileExists(t, filepath.Join(sub, "stamp"), true)
}

func TestUserMenuRunForEachFlatUsesPanelDirectory(t *testing.T) {
	root := t.TempDir()
	cfgDir := t.TempDir()
	for _, f := range []string{"lantern.txt", "willow.txt"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeUserMenuFile(t, filepath.Join(cfgDir, config.DefaultUserMenuFileName),
		"[each]\nkey = \"e\"\ntitle = \"Each\"\ncommand = \"touch stamp && echo %f\"\nrun_for_each = [\"files\"]\n")
	app := testUserMenuApp(t, root, cfgDir)
	app.model.Primary.SelectedPaths = map[string]bool{
		filepath.Join(root, "lantern.txt"): true,
		filepath.Join(root, "willow.txt"):  true,
	}

	runUserMenuKey(app, 'e')
	waitCommandsDone(t, app)

	assertFileExists(t, filepath.Join(root, "stamp"), true)
}

func TestUserMenuTreeCwdPanelRunsInPanelRoot(t *testing.T) {
	for _, mode := range []string{"", "dialog = true\n", "background = true\n"} {
		t.Run(strings.TrimSpace(mode), func(t *testing.T) {
			app, root, sub := userMenuTreeFixture(t, "[touch]\nkey = \"t\"\ntitle = \"Touch\"\ncommand = \"touch ember\"\ncwd = \"panel\"\n"+mode)
			selectEntryByName(t, app, "ember.txt")

			runUserMenuKey(app, 't')
			waitCommandsDone(t, app)

			assertFileExists(t, filepath.Join(root, "ember"), true)
			assertFileExists(t, filepath.Join(sub, "ember"), false)
		})
	}
}

func TestUserMenuTreeCwdPanelInteractiveAndDetach(t *testing.T) {
	for _, mode := range []string{"interactive", "detach"} {
		t.Run(mode, func(t *testing.T) {
			app, root, _ := userMenuTreeFixture(t, "[run]\nkey = \"g\"\ntitle = \"Run\"\ncommand = \"lazygit\"\ncwd = \"panel\"\n"+mode+" = true\n")
			var gotDir string
			prevI, prevD := userMenuInteractiveRunner, userMenuDetachRunner
			userMenuInteractiveRunner = func(_ context.Context, _ []string, dir string) error {
				gotDir = dir
				return nil
			}
			userMenuDetachRunner = func(_ []string, dir string) error {
				gotDir = dir
				return nil
			}
			t.Cleanup(func() { userMenuInteractiveRunner, userMenuDetachRunner = prevI, prevD })
			selectEntryByName(t, app, "ember.txt")

			runUserMenuKey(app, 'g')

			if gotDir != root {
				t.Fatalf("dir = %q, want %q", gotDir, root)
			}
		})
	}
}

// Macros ignore cwd: %f is the absolute path of the cursor row and %d its directory, even
// though the command runs in the panel root.
func TestUserMenuTreeCwdPanelMacrosStayAbsolute(t *testing.T) {
	app, root, sub := userMenuTreeFixture(t, "[show]\nkey = \"s\"\ntitle = \"Show\"\ncommand = \"echo %f %d > where\"\ncwd = \"panel\"\n")
	selectEntryByName(t, app, "ember.txt")

	runUserMenuKey(app, 's')
	waitCommandsDone(t, app)

	b, err := os.ReadFile(filepath.Join(root, "where"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(sub, "ember.txt") + " " + sub + "\n"; string(b) != want {
		t.Fatalf("where = %q, want %q", b, want)
	}
}

func TestUserMenuRunForEachTreeCwdPanelRunsInPanelRoot(t *testing.T) {
	app, root, sub := userMenuTreeFixture(t, "[each]\nkey = \"e\"\ntitle = \"Each\"\ncommand = \"touch stamp && echo %f >> seen\"\nrun_for_each = [\"files\"]\ncwd = \"panel\"\n")
	p := &app.model.Primary
	nested := filepath.Join(sub, "ember.txt")
	p.SelectedPaths = map[string]bool{
		filepath.Join(root, "lantern.txt"): true,
		nested:                             true,
	}

	runUserMenuKey(app, 'e')
	waitCommandsDone(t, app)

	assertFileExists(t, filepath.Join(root, "stamp"), true)
	assertFileExists(t, filepath.Join(sub, "stamp"), false)
	b, err := os.ReadFile(filepath.Join(root, "seen"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), nested) {
		t.Fatalf("seen = %q, want it to hold %q", b, nested)
	}
}

// ghostOverlay makes dir listings gain one index-only (ghost) file called name in every
// directory whose base name is inDir ("" for any), like the fileindex overlay of a browse-with panel.
func ghostOverlay(name, inDir string) panel.IndexOverlayFunc {
	return func(_ context.Context, dir pathloc.Path, entries []fsbackend.Entry) ([]fsbackend.Entry, error) {
		if inDir != "" && dir.Base() != inDir {
			return entries, nil
		}
		return append(entries, fsbackend.Entry{
			Name:        name,
			Loc:         pathloc.MustParse(pathloc.FileIndexPrefix + filepath.Join(dir.String(), name)),
			Type:        fsbackend.EntryFile,
			IndexStatus: localfs.IndexStatusMissing,
		}), nil
	}
}

// userMenuGhostApp is a flat browse-with panel over dir holding the real file "harbor.txt" and the
// ghost "phantom.txt", with a user menu whose entry "r" runs command (extra TOML lines in extra).
func userMenuGhostApp(t *testing.T, command, extra string) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	cfgDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "harbor.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeUserMenuFile(t, filepath.Join(cfgDir, config.DefaultUserMenuFileName),
		"[run]\nkey = \"r\"\ntitle = \"Run\"\ncommand = \""+command+"\"\n"+extra)
	app := testUserMenuApp(t, dir, cfgDir)
	p := &app.model.Primary
	p.WithIndex = true
	p.IndexOverlay = ghostOverlay("phantom.txt", "")
	p.ScheduleAsyncLoad = nil // list synchronously, through the overlay
	if err := p.Load(dir); err != nil {
		t.Fatal(err)
	}
	selectEntryByName(t, app, "phantom.txt")
	return app, dir
}

func assertUserCommandRefused(t *testing.T, app *App, wantMsg string) {
	t.Helper()
	if len(app.model.CommandsList) != 0 {
		t.Fatalf("a command row was recorded: %+v", app.model.CommandsList)
	}
	if !strings.Contains(app.model.Message, wantMsg) {
		t.Fatalf("message = %q, want it to contain %q", app.model.Message, wantMsg)
	}
}

func TestUserMenuRefusesGhostCursorRow(t *testing.T) {
	app, dir := userMenuGhostApp(t, "mkdir -p %f", "")
	runUserMenuKey(app, 'r')
	assertUserCommandRefused(t, app, "Only in the index: phantom.txt")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory gained entries: %v", entries)
	}
}

func TestUserMenuRefusesGhostKeepOpenWithoutRanToast(t *testing.T) {
	app, _ := userMenuGhostApp(t, "echo %f", "background = true\nkeep_open = true\n")
	runUserMenuKey(app, 'r')
	assertUserCommandRefused(t, app, "Only in the index: phantom.txt")
	if strings.Contains(app.model.Message, "Ran") {
		t.Fatalf("message = %q claims the command ran", app.model.Message)
	}
}

func TestUserMenuRefusesGhostCursorWithTagsAndKeepsTags(t *testing.T) {
	app, dir := userMenuGhostApp(t, "echo %t %f", "detach = true\n")
	ran := false
	stubDetach(t, func([]string, string) error { ran = true; return nil })
	app.model.Primary.SelectedPaths = map[string]bool{
		filepath.Join(dir, "harbor.txt"):                            true,
		pathloc.FileIndexPrefix + filepath.Join(dir, "phantom.txt"): true,
	}

	runUserMenuKey(app, 'r')

	assertUserCommandRefused(t, app, "Only in the index: phantom.txt")
	if ran {
		t.Fatal("command ran")
	}
	if got := len(app.model.Primary.SelectedPaths); got != 2 {
		t.Fatalf("tags = %d, want 2 (kept)", got)
	}
}

func TestUserMenuRefusesPercentTWithTaggedGhost(t *testing.T) {
	app, dir := userMenuGhostApp(t, "echo %t", "detach = true\n")
	ran := false
	stubDetach(t, func([]string, string) error { ran = true; return nil })
	selectEntryByName(t, app, "harbor.txt")
	tags := map[string]bool{
		filepath.Join(dir, "harbor.txt"):                            true,
		pathloc.FileIndexPrefix + filepath.Join(dir, "phantom.txt"): true,
	}
	app.model.Primary.SelectedPaths = maps.Clone(tags)

	runUserMenuKey(app, 'r')

	assertUserCommandRefused(t, app, "Only in the index: phantom.txt")
	if ran {
		t.Fatal("command ran")
	}
	if !maps.Equal(app.model.Primary.SelectedPaths, tags) {
		t.Fatalf("tags = %v, want them intact: %v", app.model.Primary.SelectedPaths, tags)
	}
}

// The cursor row is not part of %t: real tags run even with a ghost row under the cursor.
func TestUserMenuPercentTRealTagsGhostCursorRuns(t *testing.T) {
	app, dir := userMenuGhostApp(t, "echo %t", "detach = true\n")
	var gotArgv []string
	stubDetach(t, func(argv []string, _ string) error { gotArgv = argv; return nil })
	app.model.Primary.SelectedPaths = map[string]bool{filepath.Join(dir, "harbor.txt"): true}

	runUserMenuKey(app, 'r')

	if want := []string{"echo", filepath.Join(dir, "harbor.txt")}; !slices.Equal(gotArgv, want) {
		t.Fatalf("argv = %q, want %q", gotArgv, want)
	}
}

func TestUserMenuNoPercentTLeavesTaggedGhostAlone(t *testing.T) {
	app, dir := userMenuGhostApp(t, "touch x", "detach = true\n")
	ran := false
	stubDetach(t, func([]string, string) error { ran = true; return nil })
	selectEntryByName(t, app, "harbor.txt")
	tags := map[string]bool{
		filepath.Join(dir, "harbor.txt"):                            true,
		pathloc.FileIndexPrefix + filepath.Join(dir, "phantom.txt"): true,
	}
	app.model.Primary.SelectedPaths = maps.Clone(tags)

	runUserMenuKey(app, 'r')

	if !ran {
		t.Fatalf("command did not run: %q", app.model.Message)
	}
	if !maps.Equal(app.model.Primary.SelectedPaths, tags) {
		t.Fatalf("tags = %v, want them untouched: %v", app.model.Primary.SelectedPaths, tags)
	}
}

func TestUserMenuGhostCursorRunsCommandWithoutMacros(t *testing.T) {
	app, _ := userMenuGhostApp(t, "touch x", "detach = true\n")
	var gotDir string
	stubDetach(t, func(_ []string, dir string) error { gotDir = dir; return nil })

	runUserMenuKey(app, 'r')

	if gotDir != app.model.Primary.PathString() {
		t.Fatalf("detach dir = %q, want the panel directory", gotDir)
	}
}

func TestUserMenuRefusesGhostOnOtherPanel(t *testing.T) {
	app, dir := userMenuGhostApp(t, "diff %f %F", "background = true\nkeep_open = true\n")
	other := &app.model.Secondary
	other.WithIndex = true
	other.IndexOverlay = ghostOverlay("phantom.txt", "")
	other.ScheduleAsyncLoad = nil
	if err := other.Load(dir); err != nil {
		t.Fatal(err)
	}
	selectEntryByName(t, app, "harbor.txt")
	other.SelectVisibleEntry("phantom.txt")

	runUserMenuKey(app, 'r')

	assertUserCommandRefused(t, app, "Only in the index: phantom.txt")
	if strings.Contains(app.model.Message, "Ran") {
		t.Fatalf("message = %q claims the command ran", app.model.Message)
	}
}

func TestUserMenuRunForEachRefusesGhost(t *testing.T) {
	app, dir := userMenuGhostApp(t, "mkdir -p %f", "run_for_each = [\"files\"]\n")
	app.model.Primary.SelectedPaths = map[string]bool{
		filepath.Join(dir, "harbor.txt"):                            true,
		pathloc.FileIndexPrefix + filepath.Join(dir, "phantom.txt"): true,
	}
	runUserMenuKey(app, 'r')
	assertUserCommandRefused(t, app, "Only in the index: phantom.txt")
	if got := len(app.model.Primary.SelectedPaths); got != 2 {
		t.Fatalf("tags = %d, want 2 (kept)", got)
	}
}

func TestUserMenuRunsRealRowOnBrowseWithPanel(t *testing.T) {
	app, _ := userMenuGhostApp(t, "echo %f", "detach = true\n")
	var gotDir string
	stubDetach(t, func(_ []string, dir string) error { gotDir = dir; return nil })
	selectEntryByName(t, app, "harbor.txt")

	runUserMenuKey(app, 'r')

	if gotDir != app.model.Primary.PathString() {
		t.Fatalf("detach dir = %q, want the panel directory", gotDir)
	}
}

func TestUserMenuRefusesNonLocalPanels(t *testing.T) {
	for _, path := range []string{"sftp://paras@example.invalid/srv", pathloc.FileIndexPrefix + "/srv/orchard"} {
		app, _ := userMenuGhostApp(t, "echo %d", "detach = true\n")
		ran := false
		stubDetach(t, func([]string, string) error { ran = true; return nil })
		app.model.Primary.Path = pathloc.MustParse(path)

		runUserMenuKey(app, 'r')

		assertUserCommandRefused(t, app, "User commands run on local panels only")
		if ran {
			t.Fatalf("%s: command ran", path)
		}
	}
}

func TestUserMenuRefusesGhostRowInTreeLayout(t *testing.T) {
	app, _, _ := userMenuTreeFixtureWith(t, "[run]\nkey = \"r\"\ntitle = \"Run\"\ncommand = \"mkdir -p %f\"\n",
		ghostOverlay("phantom.txt", "orchard"))
	selectEntryByName(t, app, "phantom.txt")

	runUserMenuKey(app, 'r')

	assertUserCommandRefused(t, app, "Only in the index: phantom.txt")
}

func TestUserMenuTreeGhostCursorRefusedByCursorDirectory(t *testing.T) {
	app, _, _ := userMenuTreeFixtureWith(t, "[run]\nkey = \"r\"\ntitle = \"Run\"\ncommand = \"touch x\"\n",
		ghostOverlay("phantom.txt", "orchard"))
	selectEntryByName(t, app, "phantom.txt")

	runUserMenuKey(app, 'r')

	assertUserCommandRefused(t, app, "Only in the index: phantom.txt")
}

func TestUserMenuTreeGhostCursorRunsInPanelDirectory(t *testing.T) {
	app, root, sub := userMenuTreeFixtureWith(t, "[run]\nkey = \"r\"\ntitle = \"Run\"\ncommand = \"touch x\"\ncwd = \"panel\"\n",
		ghostOverlay("phantom.txt", "orchard"))
	selectEntryByName(t, app, "phantom.txt")

	runUserMenuKey(app, 'r')
	waitCommandsDone(t, app)

	assertFileExists(t, filepath.Join(root, "x"), true)
	assertFileExists(t, filepath.Join(sub, "x"), false)
}

func TestUserMenuRunForEachRealTagsGhostCursorRuns(t *testing.T) {
	app, dir := userMenuGhostApp(t, "touch %f.copy", "run_for_each = [\"files\"]\ncwd = \"panel\"\n")
	app.model.Primary.SelectedPaths = map[string]bool{filepath.Join(dir, "harbor.txt"): true}

	runUserMenuKey(app, 'r')
	waitCommandsDone(t, app)

	assertFileExists(t, filepath.Join(dir, "harbor.txt.copy"), true)
}

func TestRunForEachDialogRefusesGhostAtOpen(t *testing.T) {
	app, dir := userMenuGhostApp(t, "echo", "")
	app.commandsCtrl.OpenRunForEachDialog()
	if app.model.FileDialog.Open {
		t.Fatal("dialog opened over a ghost cursor row")
	}
	if !strings.Contains(app.model.Message, "Only in the index: phantom.txt") {
		t.Fatalf("message = %q", app.model.Message)
	}

	selectEntryByName(t, app, "harbor.txt")
	app.model.Primary.SelectedPaths = map[string]bool{pathloc.FileIndexPrefix + filepath.Join(dir, "phantom.txt"): true}
	app.commandsCtrl.OpenRunForEachDialog()
	if app.model.FileDialog.Open {
		t.Fatal("dialog opened over a tagged ghost")
	}

	app.model.Primary.SelectedPaths = nil
	app.commandsCtrl.OpenRunForEachDialog()
	if !app.model.FileDialog.Open {
		t.Fatal("dialog should open on a real row")
	}
}

func TestRunForEachDialogOpensOnRealTagsWithGhostCursor(t *testing.T) {
	app, dir := userMenuGhostApp(t, "echo", "")
	app.model.Primary.SelectedPaths = map[string]bool{filepath.Join(dir, "harbor.txt"): true}
	app.commandsCtrl.OpenRunForEachDialog()
	if !app.model.FileDialog.Open {
		t.Fatalf("dialog should open on real tags: %q", app.model.Message)
	}
}

func TestRunForEachDialogRefusesGhostOnOtherPanelOnceTyped(t *testing.T) {
	app, dir := userMenuGhostApp(t, "echo", "")
	other := &app.model.Secondary
	other.WithIndex = true
	other.IndexOverlay = ghostOverlay("phantom.txt", "")
	other.ScheduleAsyncLoad = nil
	if err := other.Load(dir); err != nil {
		t.Fatal(err)
	}
	other.SelectVisibleEntry("phantom.txt")
	selectEntryByName(t, app, "harbor.txt")
	app.commandsCtrl.OpenRunForEachDialog()
	if !app.model.FileDialog.Open {
		t.Fatalf("dialog should open: %q", app.model.Message)
	}

	app.model.FileDialog.Fields[0].Value = "diff %f %F"
	app.commandsCtrl.ExecuteRunForEach()

	fd := app.model.FileDialog
	if !fd.Open || fd.RunForEachCommandError != "Only in the index: phantom.txt" || !fd.Fields[0].InputInvalid {
		t.Fatalf("open=%v error=%q invalid=%v", fd.Open, fd.RunForEachCommandError, fd.Fields[0].InputInvalid)
	}
	if len(app.model.CommandsList) != 0 {
		t.Fatalf("a command row was recorded: %+v", app.model.CommandsList)
	}

	app.model.FileDialog.Fields[0].Value = "echo %f"
	app.commandsCtrl.RecomputeRunForEachValidation()
	if got := app.model.FileDialog.RunForEachCommandError; got != "" {
		t.Fatalf("error = %q for a command without %%F", got)
	}
}

func TestRunForEachDialogRefusesNonLocalPanel(t *testing.T) {
	app, _ := userMenuGhostApp(t, "echo", "")
	app.model.Primary.Path = pathloc.MustParse("sftp://paras@example.invalid/srv")
	app.commandsCtrl.OpenRunForEachDialog()
	if app.model.FileDialog.Open || !strings.Contains(app.model.Message, "local panels only") {
		t.Fatalf("open=%v message=%q", app.model.FileDialog.Open, app.model.Message)
	}
}
