package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// The theme an app starts with must reach the screen's base background exactly like a theme
// switched to later (setStyles is the one path for both).
func TestStartupThemeSetsBaseBackground(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lantern.toml")
	if err := os.WriteFile(path, theme.TestThemeBytes(t, map[string]string{"roles.bg": `"#303446"`}), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	th, err := theme.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	f, _, _ := newRecordingFrame(t, 80, 24)
	if _, err := NewWithOptions(f, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: config.Default(),
		Theme:  th,
	}); err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	if want := tcell.NewRGBColor(0x30, 0x34, 0x46); f.base != want {
		t.Fatalf("base background after startup = %v, want the theme's bg %v", f.base, want)
	}
}
