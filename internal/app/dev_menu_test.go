package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestDevMenuShowInfoToast(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	defer screen.Fini()

	styles, paths := loadTestTheme(t)
	app, err := NewWithOptions(screen, Options{
		CWD:     func() (string, error) { return t.TempDir(), nil },
		Config:  config.Default(),
		Theme:   styles,
		Paths:   paths,
		DevMode: true,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	app.dispatch(keymap.ActionAppOpenMenu)
	if !app.openMenuByShortcut('v') {
		t.Fatal("expected Dev menu shortcut to open pulldown")
	}
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone))
	if quit {
		t.Fatal("Show info must not quit")
	}
	if app.model.Message != "Example info message" {
		t.Fatalf("Message = %q, want info toast text without render padding", app.model.Message)
	}
	if app.model.MessageUrgency != ui.MessageUrgencyInfo {
		t.Fatalf("MessageUrgency = %v, want info", app.model.MessageUrgency)
	}
}
