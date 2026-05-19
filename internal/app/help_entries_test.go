package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestBuildHelpEntriesIncludesCrossPanelOpenActions(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	a, err := New(screen, func() (string, error) { return t.TempDir(), nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	entries := a.buildHelpEntries()
	var keysOpenDir, keysOpenCwd string
	for _, e := range entries {
		switch e.ActionID {
		case "panel.open-dir-in-other":
			keysOpenDir = e.Keys
		case "panel.open-active-path-in-other":
			keysOpenCwd = e.Keys
		}
	}
	if keysOpenDir == "" {
		t.Fatal("help missing panel.open-dir-in-other (expected default Alt+O)")
	}
	if !strings.Contains(keysOpenDir, "Alt") || !strings.Contains(keysOpenDir, "O") {
		t.Fatalf("unexpected keys display for open-dir-in-other: %q", keysOpenDir)
	}
	if keysOpenCwd == "" {
		t.Fatal("help missing panel.open-active-path-in-other (expected default Alt+I)")
	}
	if !strings.Contains(keysOpenCwd, "Alt") || !strings.Contains(keysOpenCwd, "I") {
		t.Fatalf("unexpected keys display for open-active-path-in-other: %q", keysOpenCwd)
	}
}
