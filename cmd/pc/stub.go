package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/pools"
	"github.com/paranoidi/paras-commander/internal/usermenu"
)

// writeConfigDirStubs writes example configuration files into dir.
// Existing files are left unchanged; a notice is printed to stderr for each skip.
func writeConfigDirStubs(dir string, stderr io.Writer) error {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "" {
		return fmt.Errorf("config stub directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config directory %q: %w", dir, err)
	}

	configPath := filepath.Join(dir, config.ConfigFileName())
	keybindingsPath := filepath.Join(dir, config.KeybindingsFileName())
	menuPath := filepath.Join(dir, config.DefaultUserMenuFileName)
	metaPath := filepath.Join(dir, config.DefaultMetaFileName)
	poolsPath := filepath.Join(dir, config.DefaultPoolsFileName)

	type stub struct {
		path  string
		write func() (created bool, err error)
	}
	stubs := []stub{
		{configPath, func() (bool, error) { return writeEncodedConfigStub(configPath) }},
		{keybindingsPath, func() (bool, error) { return writeEncodedKeybindingsStub(keybindingsPath) }},
		{menuPath, func() (bool, error) { return usermenu.WriteMenuStub(menuPath) }},
		{metaPath, func() (bool, error) { return metacmds.WriteMetaStub(metaPath) }},
		{poolsPath, func() (bool, error) { return pools.WritePoolsStub(poolsPath) }},
	}

	for _, s := range stubs {
		created, err := s.write()
		if err != nil {
			return err
		}
		if !created {
			if _, err := fmt.Fprintf(stderr, "pc: skipping existing file: %s\n", s.path); err != nil {
				return fmt.Errorf("write skip notice: %w", err)
			}
		}
	}
	return nil
}

func writeEncodedConfigStub(path string) (bool, error) {
	return writeEncodedStub(path, func(w io.Writer) error {
		return config.EncodeDefaultStub(w)
	})
}

func writeEncodedKeybindingsStub(path string) (bool, error) {
	return writeEncodedStub(path, func(w io.Writer) error {
		return keymap.EncodeDefaultStub(w)
	})
}

func writeEncodedStub(path string, encode func(io.Writer) error) (bool, error) {
	path = filepath.Clean(path)
	if _, statErr := os.Stat(path); statErr == nil {
		return false, nil
	} else if !os.IsNotExist(statErr) {
		return false, fmt.Errorf("stat %q: %w", path, statErr)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create directory for %q: %w", path, err)
	}
	var buf bytes.Buffer
	if err := encode(&buf); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return false, fmt.Errorf("write %q: %w", path, err)
	}
	return true, nil
}

func resolveConfigStubDir(arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		paths, err := config.DefaultPaths()
		if err != nil {
			return "", err
		}
		return paths.ConfigDir, nil
	}
	return expandHomePath(arg)
}

func expandHomePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("config stub path is empty")
	}
	if p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return home, nil
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}
