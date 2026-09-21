//go:build linux

package app

import (
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	commandsctrl "github.com/paranoidi/paras-commander/internal/apphandler/commands"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/subshell"
	"golang.org/x/sys/unix"
)

// newTerminalPanelApp returns an app with the persistent subshell enabled on a stub
// /bin/sh (PTYs work headlessly; only RunVisible needs a real /dev/tty).
func newTerminalPanelApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("SHELL", "/bin/sh")
	app := newApp(t, newScreen(t, 100, 40), t.TempDir())
	app.config.Shell.Persistent = true
	t.Cleanup(app.closeSubshell)
	return app
}

func TestTerminalPanelToggleVisibleShowsWithoutFocus(t *testing.T) {
	app := newTerminalPanelApp(t)

	app.toggleTerminalPanelVisible()
	tp := &app.model.TerminalPanel
	if !tp.Visible || tp.Focused || app.terminalFeed == nil || tp.Drawer == nil {
		t.Fatalf("toggle-panel from hidden: want visible+unfocused with feed, got visible=%v focused=%v feed=%v", tp.Visible, tp.Focused, app.terminalFeed)
	}

	app.toggleTerminalPanelVisible()
	if tp.Visible || tp.Focused || tp.Drawer != nil {
		t.Fatalf("toggle-panel again: want hidden, got visible=%v focused=%v", tp.Visible, tp.Focused)
	}
	if app.terminalFeed == nil {
		t.Fatal("feed must stay alive across hide/show cycles")
	}
	if app.subshell == nil || !app.subshell.Alive() {
		t.Fatal("closing the panel must not kill the shell session")
	}
}

func TestTerminalPanelToggleVisibleHidesFromFocused(t *testing.T) {
	app := newTerminalPanelApp(t)
	tp := &app.model.TerminalPanel

	app.toggleTerminalPanelFocus()
	if !tp.Visible || !tp.Focused {
		t.Fatalf("focus toggle from hidden: want open+focused, got visible=%v focused=%v", tp.Visible, tp.Focused)
	}

	app.toggleTerminalPanelVisible()
	if tp.Visible || tp.Focused {
		t.Fatalf("toggle-panel while focused: want hidden, got visible=%v focused=%v", tp.Visible, tp.Focused)
	}
	if app.terminalFeed == nil {
		t.Fatal("feed must stay alive after hiding a focused panel")
	}
}

func TestTerminalPanelFocusOpensWhenHidden(t *testing.T) {
	app := newTerminalPanelApp(t)

	app.toggleTerminalPanelFocus()
	tp := &app.model.TerminalPanel
	if !tp.Visible || !tp.Focused || app.terminalFeed == nil || tp.Drawer == nil {
		t.Fatalf("focus toggle from hidden: want open+focused with feed, got visible=%v focused=%v feed=%v", tp.Visible, tp.Focused, app.terminalFeed)
	}
}

func TestTerminalPanelFocusRoundTripKeepsVisible(t *testing.T) {
	app := newTerminalPanelApp(t)
	tp := &app.model.TerminalPanel

	app.toggleTerminalPanelFocus()
	if !tp.Visible || !tp.Focused {
		t.Fatalf("first focus toggle: want open+focused, got visible=%v focused=%v", tp.Visible, tp.Focused)
	}

	app.toggleTerminalPanelFocus()
	if !tp.Visible || tp.Focused {
		t.Fatalf("second focus toggle: want visible+unfocused, got visible=%v focused=%v", tp.Visible, tp.Focused)
	}
	if app.terminalFeed == nil {
		t.Fatal("feed must keep running while unfocused")
	}

	app.toggleTerminalPanelFocus()
	if !tp.Visible || !tp.Focused {
		t.Fatalf("third focus toggle: want focused again, got visible=%v focused=%v", tp.Visible, tp.Focused)
	}
	if app.subshell == nil || !app.subshell.Alive() {
		t.Fatal("focus toggling must not kill the shell session")
	}
}

func TestTerminalPanelFocusedKeysBypassGlobals(t *testing.T) {
	app := newTerminalPanelApp(t)
	app.toggleTerminalPanelFocus()
	if !app.terminalPanelHasKeyFocus() {
		t.Fatal("panel should have key focus after open")
	}

	// F10 must reach the shell (htop's quit key), not open the app quit flow.
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))
	if quit || app.model.QuitConfirm.Open {
		t.Fatalf("F10 while terminal focused: quit=%v confirmOpen=%v, want neither", quit, app.model.QuitConfirm.Open)
	}
	// F1 must not open help.
	if _, _ = app.handleKey(tcell.NewEventKey(tcell.KeyF1, 0, tcell.ModNone)); app.model.HelpView.Open {
		t.Fatal("F1 while terminal focused must not open help")
	}

	// Alt+P (terminal.focus in [terminal]) unfocuses.
	if _, _ = app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModAlt)); app.model.TerminalPanel.Focused {
		t.Fatal("M-p while focused should unfocus the panel")
	}

	// Unfocused: F10 goes back to the global quit flow.
	quit, _ = app.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))
	if !quit && !app.model.QuitConfirm.Open {
		t.Fatal("F10 with panel unfocused should reach the quit flow")
	}
}

func TestTerminalPanelKeystrokesReachShell(t *testing.T) {
	app := newTerminalPanelApp(t)
	app.toggleTerminalPanelFocus()

	for _, r := range "echo pomegranate\r" {
		key := tcell.KeyRune
		if r == '\r' {
			key = tcell.KeyEnter
		}
		_, _ = app.handleKey(tcell.NewEventKey(key, r, tcell.ModNone))
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if screenText := terminalPanelText(app); screenText != "" && containsOutput(screenText, "pomegranate") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("typed command output never reached the emulator:\n%s", terminalPanelText(app))
}

func TestTerminalPanelShowsOutputFromBeforeFirstOpen(t *testing.T) {
	app := newTerminalPanelApp(t)

	// The feed must start with the subshell so a full-screen session's output
	// is captured even though the embedded panel has never been opened.
	if _, ok := app.ensureSubshell(t.TempDir()); !ok {
		t.Fatal("ensureSubshell failed")
	}
	if app.terminalFeed == nil {
		t.Fatal("feed must start with the subshell, before the panel opens")
	}
	if _, err := app.subshell.WritePTY([]byte("echo persimmon\n")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if containsOutput(terminalPanelText(app), "persimmon") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	app.toggleTerminalPanelVisible()
	if !app.model.TerminalPanel.Visible {
		t.Fatal("panel did not open")
	}
	if !containsOutput(terminalPanelText(app), "persimmon") {
		t.Fatalf("output from before first open is missing:\n%s", terminalPanelText(app))
	}
}

func TestTerminalPanelResizeClamps(t *testing.T) {
	app := newTerminalPanelApp(t)
	app.toggleTerminalPanelVisible()
	tp := &app.model.TerminalPanel

	start := tp.Rows
	app.growTerminalPanel()
	if tp.Rows != start+1 {
		t.Fatalf("grow: rows = %d, want %d", tp.Rows, start+1)
	}
	app.shrinkTerminalPanel()
	if tp.Rows != start {
		t.Fatalf("shrink: rows = %d, want %d", tp.Rows, start)
	}
	tp.Rows = config.MinShellTerminalPanelHeight
	app.shrinkTerminalPanel()
	if tp.Rows != config.MinShellTerminalPanelHeight {
		t.Fatalf("shrink at min: rows = %d, want clamp at %d", tp.Rows, config.MinShellTerminalPanelHeight)
	}
}

func terminalPanelText(app *App) string {
	if app.terminalFeed == nil {
		return ""
	}
	var out []rune
	lastY := -1
	_, _, _ = app.terminalFeed.Draw(tcell.StyleDefault, func(x, y int, r rune, _ tcell.Style) {
		if y != lastY {
			out = append(out, '\n')
			lastY = y
		}
		out = append(out, r)
	})
	return string(out)
}

func containsOutput(haystack, needle string) bool {
	count := 0
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			count++
		}
	}
	// Echo of the typed line plus the command's own output.
	return count >= 2
}

func TestResizeTerminalFeedToLayoutUpdatesRunForEachPTY(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 100, 40)
	app := newApp(t, screen, dir)
	stopPump := startAppWakePump(t, app, screen)
	t.Cleanup(func() {
		if sub, _, ok := app.commandsCtrl.ActivePTYSession(); ok && sub != nil {
			_ = sub.Close()
		}
	})

	app.commandsCtrl.StartRunForEachBatch(commandsctrl.RunForEachBatchSpec{
		Entries:   []localfs.Entry{{Name: "willow", Path: dir, Type: localfs.EntryDirectory}},
		AllowDirs: true,
		WorkDir:   dir,
		PTY:       true,
		BuildItem: func(localfs.Entry) (commandsctrl.RunForEachBuiltItem, error) {
			return commandsctrl.RunForEachBuiltItem{Argv: []string{"sleep", "60"}, UserLine: "sleep 60"}, nil
		},
	})

	var sub *subshell.Subshell
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var ok bool
		sub, _, ok = app.commandsCtrl.ActivePTYSession()
		if ok && sub != nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	stopPump()
	if sub == nil {
		t.Fatal("timed out waiting for run-for-each ActivePTYSession")
	}
	if !app.model.TerminalPanel.Visible {
		t.Fatal("run-for-each PTY should own a visible terminal panel")
	}
	if app.terminalFeed != nil {
		t.Fatal("persistent terminalFeed must stay nil so resize uses ActivePTYSession")
	}

	before, err := unix.IoctlGetWinsize(sub.PTYFd(), unix.TIOCGWINSZ)
	if err != nil {
		t.Fatal(err)
	}

	screen.SetSize(140, 50)
	wantCols, wantRows, ok := app.terminalPanelContentDims()
	if !ok {
		t.Fatal("layout omitted the terminal strip after screen resize")
	}
	app.resizeTerminalFeedToLayout()

	after, err := unix.IoctlGetWinsize(sub.PTYFd(), unix.TIOCGWINSZ)
	if err != nil {
		t.Fatal(err)
	}
	if int(after.Col) != wantCols || int(after.Row) != wantRows {
		t.Fatalf("PTY size after layout resize = %dx%d, want %dx%d (before %dx%d)",
			after.Col, after.Row, wantCols, wantRows, before.Col, before.Row)
	}
}

func TestPostTerminalWakeClearsPendingWhenPostEventFails(t *testing.T) {
	dir := t.TempDir()
	inner := newScreen(t, 80, 24)
	app := newApp(t, inner, dir)
	screen := &failOncePostScreen{SimulationScreen: inner, failsLeft: 1}
	app.screen = screen

	app.postTerminalWake()
	if app.terminalWakePending.Load() {
		t.Fatal("failed PostEvent must clear terminalWakePending")
	}

	app.postTerminalWake()
	if !app.terminalWakePending.Load() {
		t.Fatal("successful PostEvent should leave pending set until handleTerminalWake")
	}
	if !inner.HasPendingEvent() {
		t.Fatal("second post should queue a terminal wake")
	}
	ev := inner.PollEvent()
	interrupt, ok := ev.(*tcell.EventInterrupt)
	if !ok {
		t.Fatalf("queued event is %T, want *tcell.EventInterrupt", ev)
	}
	if _, ok := interrupt.Data().(terminalWakePayload); !ok {
		t.Fatalf("payload is %T, want terminalWakePayload", interrupt.Data())
	}
}

type failOncePostScreen struct {
	tcell.SimulationScreen
	failsLeft int
}

func (s *failOncePostScreen) PostEvent(ev tcell.Event) error {
	if s.failsLeft > 0 {
		s.failsLeft--
		return tcell.ErrEventQFull
	}
	return s.SimulationScreen.PostEvent(ev)
}

func startAppWakePump(t *testing.T, app *App, screen tcell.SimulationScreen) func() {
	t.Helper()
	stop := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once
	go func() {
		defer close(done)
		for {
			ev := screen.PollEvent()
			select {
			case <-stop:
				return
			default:
			}
			interrupt, ok := ev.(*tcell.EventInterrupt)
			if !ok || interrupt.Data() == nil {
				continue
			}
			app.handleInterruptPayload(interrupt.Data())
		}
	}()
	stopPump := func() {
		once.Do(func() {
			close(stop)
			_ = screen.PostEvent(tcell.NewEventInterrupt(nil))
			<-done
		})
	}
	t.Cleanup(stopPump)
	return stopPump
}
