package metacmds

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/filetrust"
)

// ResolveMetaGlobalPath returns the path to the user-wide meta.toml (may not exist).
func ResolveMetaGlobalPath(cfg config.Config, homeDir, configDir string) string {
	if f := strings.TrimSpace(cfg.Meta.File); f != "" {
		return expandHome(f, homeDir)
	}
	if strings.TrimSpace(configDir) == "" {
		return ""
	}
	return filepath.Join(configDir, config.DefaultMetaFileName)
}

// ResolveMetaTOML finds the first readable trusted meta.toml: cwd candidates then global.
// Returns empty path when none found.
// Warnings describe skipped untrusted or missing hops (informational).
func ResolveMetaTOML(cfg config.Config, homeDir, configDir, panelPath string) (path string, warnings []string) {
	panelPath = filepath.Clean(panelPath)
	for _, base := range cfg.Meta.LocalNames {
		base = strings.TrimSpace(base)
		if base == "" {
			continue
		}
		cand := filepath.Join(panelPath, base)
		if _, err := os.Stat(cand); err != nil {
			continue
		}
		if why := filetrust.Check(cand); why != "" {
			warnings = append(warnings, fmt.Sprintf("meta: ignoring untrusted file %q: %s", cand, why))
			continue
		}
		return cand, warnings
	}
	global := ResolveMetaGlobalPath(cfg, homeDir, configDir)
	if global == "" {
		return "", warnings
	}
	if _, err := os.Stat(global); err != nil {
		return "", warnings
	}
	if why := filetrust.Check(global); why != "" {
		warnings = append(warnings, fmt.Sprintf("meta: ignoring untrusted file %q: %s", global, why))
		return "", warnings
	}
	return global, warnings
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
