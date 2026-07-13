//go:build linux

package app

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
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
