package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
)

func TestWriteConfigDirStubsCreatesAllFiles(t *testing.T) {
	dir := t.TempDir()
	var stderr bytes.Buffer
	if err := writeConfigDirStubs(dir, &stderr); err != nil {
		t.Fatalf("writeConfigDirStubs: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on fresh dir", stderr.String())
	}
	for _, name := range []string{
		config.ConfigFileName(),
		config.KeybindingsFileName(),
		config.DefaultUserMenuFileName,
		config.DefaultMetaFileName,
		config.DefaultPoolsFileName,
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
	}
	cfg, err := config.LoadFromPaths(config.Paths{
		ConfigFile:      filepath.Join(dir, config.ConfigFileName()),
		KeybindingsFile: filepath.Join(dir, config.KeybindingsFileName()),
	})
	if err != nil {
		t.Fatalf("LoadFromPaths config: %v", err)
	}
	if cfg.Theme != config.ThemeDefault {
		t.Fatalf("Theme = %q, want %q", cfg.Theme, config.ThemeDefault)
	}
	bundle, err := keymap.LoadFromPaths(config.Paths{
		ConfigFile:      filepath.Join(dir, config.ConfigFileName()),
		KeybindingsFile: filepath.Join(dir, config.KeybindingsFileName()),
	})
	if err != nil {
		t.Fatalf("LoadFromPaths keymap: %v", err)
	}
	if bundle == nil || bundle.Global == nil {
		t.Fatal("keymap bundle missing global map")
	}
}

func TestWriteConfigDirStubsSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, config.ConfigFileName())
	if err := os.WriteFile(configPath, []byte("theme = \"custom\"\n"), 0o644); err != nil {
		t.Fatalf("write existing config: %v", err)
	}
	var stderr bytes.Buffer
	if err := writeConfigDirStubs(dir, &stderr); err != nil {
		t.Fatalf("writeConfigDirStubs: %v", err)
	}
	if !strings.Contains(stderr.String(), configPath) {
		t.Fatalf("stderr = %q, want skip notice for %s", stderr.String(), configPath)
	}
	b, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(b) != "theme = \"custom\"\n" {
		t.Fatalf("config.toml was modified: %q", string(b))
	}
}

func TestRunConfigStubFlag(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	if err := run([]string{"-config-stub", dir}, &stderr, &stdout); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, config.ConfigFileName())); err != nil {
		t.Fatalf("config stub not written: %v", err)
	}
}

func TestResolveConfigStubDirExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	got, err := expandHomePath("~/pc-stub-test")
	if err != nil {
		t.Fatalf("expandHomePath: %v", err)
	}
	want := filepath.Join(home, "pc-stub-test")
	if got != want {
		t.Fatalf("expandHomePath = %q, want %q", got, want)
	}
}
