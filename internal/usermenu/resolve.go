package usermenu

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/config"
)

// MenuFileTrusted reports whether path is safe to load (MC menu_file_own semantics).
// Owner must be root or current euid; file must not be group- or world-writable.
func MenuFileTrusted(path string) bool {
	st, err := os.Lstat(path)
	if err != nil || st.IsDir() {
		return false
	}
	if st.Mode().Perm()&0o022 != 0 {
		return false
	}
	return fileOwnerTrusted(st)
}

func expandHome(p, home string) string {
	p = filepath.Clean(strings.TrimSpace(p))
	if home == "" {
		return p
	}
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

// ResolveUserMenuGlobalPath returns the path to the user-wide menu.toml (may not exist).
func ResolveUserMenuGlobalPath(cfg config.Config, homeDir, configDir string) string {
	if f := strings.TrimSpace(cfg.UserMenu.File); f != "" {
		return expandHome(f, homeDir)
	}
	if strings.TrimSpace(configDir) == "" {
		return ""
	}
	return filepath.Join(configDir, config.DefaultUserMenuFileName)
}

// ResolveMenuTOML finds the first readable trusted menu.toml: cwd candidates then global.
// Returns empty path when none found (caller may bootstrap a global menu.toml stub).
// Warnings describe skipped untrusted or missing hops (informational).
func ResolveMenuTOML(cfg config.Config, homeDir, configDir, panelPath string) (path string, warnings []string) {
	panelPath = filepath.Clean(panelPath)
	for _, base := range cfg.UserMenu.LocalNames {
		base = strings.TrimSpace(base)
		if base == "" {
			continue
		}
		cand := filepath.Join(panelPath, base)
		if _, err := os.Stat(cand); err != nil {
			continue
		}
		if !MenuFileTrusted(cand) {
			warnings = append(warnings, fmt.Sprintf("user menu: ignoring untrusted file %q", cand))
			continue
		}
		return cand, warnings
	}
	global := ResolveUserMenuGlobalPath(cfg, homeDir, configDir)
	if global == "" {
		return "", warnings
	}
	if _, err := os.Stat(global); err != nil {
		return "", warnings
	}
	if !MenuFileTrusted(global) {
		warnings = append(warnings, fmt.Sprintf("user menu: ignoring untrusted file %q", global))
		return "", warnings
	}
	return global, warnings
}
