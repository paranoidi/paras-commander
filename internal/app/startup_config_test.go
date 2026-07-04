package app

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestLoadUserStartupConfigBrokenTheme(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	broken := theme.TestThemeBytesNamed(t, "default", map[string]bool{
		"panel.row.file": true,
	}, nil)
	if err := os.WriteFile(filepath.Join(themesDir, "amber-glow.toml"), broken, 0o644); err != nil {
		t.Fatal(err)
	}
	paths := config.Paths{
		ConfigDir:       dir,
		ConfigFile:      filepath.Join(dir, "config.toml"),
		ThemesDir:       themesDir,
		KeybindingsFile: filepath.Join(dir, "keybindings.toml"),
	}.WithResolvedLocations()

	_, err := loadUserStartupConfig(paths)
	if err == nil {
		t.Fatal("expected error for broken theme directory")
	}
	if !strings.Contains(err.Error(), "missing required style") {
		t.Fatalf("expected missing required style error, got %v", err)
	}
}

func TestLoadUserStartupConfigInvalidConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configFile, []byte("unknown_field = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths := config.Paths{
		ConfigDir:       dir,
		ConfigFile:      configFile,
		ThemesDir:       filepath.Join(dir, "themes"),
		KeybindingsFile: filepath.Join(dir, "keybindings.toml"),
	}.WithResolvedLocations()

	_, err := loadUserStartupConfig(paths)
	if err == nil {
		t.Fatal("expected error for invalid config.toml")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestLoadUserStartupConfigInvalidKeybindings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "keybindings.toml")
	if err := os.WriteFile(keyFile, []byte("[main]\nnot_an_action = [\"Ctrl+X\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths := config.Paths{
		ConfigDir:       dir,
		ConfigFile:      filepath.Join(dir, "config.toml"),
		ThemesDir:       filepath.Join(dir, "themes"),
		KeybindingsFile: keyFile,
	}.WithResolvedLocations()

	_, err := loadUserStartupConfig(paths)
	if err == nil {
		t.Fatal("expected error for invalid keybindings.toml")
	}
	if !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("expected invalid keybindings error, got %v", err)
	}
}

func TestBuiltInStartupConfig(t *testing.T) {
	t.Parallel()
	startup, err := builtInStartupConfig()
	if err != nil {
		t.Fatalf("builtInStartupConfig: %v", err)
	}
	if startup.Config.Theme != config.ThemeDefault {
		t.Fatalf("theme = %q, want %q", startup.Config.Theme, config.ThemeDefault)
	}
	if startup.Theme.Name == "" {
		t.Fatal("expected non-empty built-in theme")
	}
	if len(startup.ThemeChoices) == 0 {
		t.Fatal("expected built-in theme choices")
	}
	if startup.Keymap == nil || startup.Keymap.Global == nil {
		t.Fatal("expected built-in keymap bundle")
	}
}

func TestResolveStartupConfigDeclinesFallback(t *testing.T) {
	dir := t.TempDir()
	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	broken := theme.TestThemeBytesNamed(t, "default", map[string]bool{
		"panel.row.file": true,
	}, nil)
	if err := os.WriteFile(filepath.Join(themesDir, "copper-lake.toml"), broken, 0o644); err != nil {
		t.Fatal(err)
	}
	paths := config.Paths{
		ConfigDir:       dir,
		ConfigFile:      filepath.Join(dir, "config.toml"),
		ThemesDir:       themesDir,
		KeybindingsFile: filepath.Join(dir, "keybindings.toml"),
	}.WithResolvedLocations()

	prev := startupDefaultsPrompt
	t.Cleanup(func() { startupDefaultsPrompt = prev })
	startupDefaultsPrompt = func(_ io.Reader, _ io.Writer, _ error) (bool, error) {
		return false, nil
	}

	_, useBuiltIn, err := resolveStartupConfig(paths)
	if err == nil {
		t.Fatal("expected load error when user declines fallback")
	}
	if useBuiltIn {
		t.Fatal("useBuiltIn should be false when user declines")
	}
}

func TestResolveStartupConfigAcceptsFallback(t *testing.T) {
	dir := t.TempDir()
	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	broken := theme.TestThemeBytesNamed(t, "default", map[string]bool{
		"panel.row.file": true,
	}, nil)
	if err := os.WriteFile(filepath.Join(themesDir, "silver-pond.toml"), broken, 0o644); err != nil {
		t.Fatal(err)
	}
	paths := config.Paths{
		ConfigDir:       dir,
		ConfigFile:      filepath.Join(dir, "config.toml"),
		ThemesDir:       themesDir,
		KeybindingsFile: filepath.Join(dir, "keybindings.toml"),
	}.WithResolvedLocations()

	prev := startupDefaultsPrompt
	t.Cleanup(func() { startupDefaultsPrompt = prev })
	startupDefaultsPrompt = func(_ io.Reader, _ io.Writer, _ error) (bool, error) {
		return true, nil
	}

	startup, useBuiltIn, err := resolveStartupConfig(paths)
	if err != nil {
		t.Fatalf("resolveStartupConfig: %v", err)
	}
	if !useBuiltIn {
		t.Fatal("useBuiltIn should be true when user accepts fallback")
	}
	if startup.Theme.Name != config.ThemeDefault {
		t.Fatalf("theme = %q, want %q", startup.Theme.Name, config.ThemeDefault)
	}
}

func TestPromptLaunchWithBuiltInDefaults(t *testing.T) {
	t.Parallel()
	loadErr := io.ErrUnexpectedEOF

	t.Run("default yes on enter", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		yes, err := promptLaunchWithBuiltInDefaults(strings.NewReader("\n"), &out, loadErr)
		if err != nil {
			t.Fatal(err)
		}
		if !yes {
			t.Fatal("expected yes for empty line")
		}
		if !strings.Contains(out.String(), "Configuration error:") {
			t.Fatalf("expected error output, got %q", out.String())
		}
		if !strings.Contains(out.String(), "Launch with built-in defaults anyway? [Y/n]:") {
			t.Fatalf("expected prompt, got %q", out.String())
		}
	})

	t.Run("no", func(t *testing.T) {
		t.Parallel()
		yes, err := promptLaunchWithBuiltInDefaults(strings.NewReader("n\n"), io.Discard, loadErr)
		if err != nil {
			t.Fatal(err)
		}
		if yes {
			t.Fatal("expected no")
		}
	})

	t.Run("yes", func(t *testing.T) {
		t.Parallel()
		yes, err := promptLaunchWithBuiltInDefaults(strings.NewReader("y\n"), io.Discard, loadErr)
		if err != nil {
			t.Fatal(err)
		}
		if !yes {
			t.Fatal("expected yes")
		}
	})
}
