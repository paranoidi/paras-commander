package pools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/config"
)

// ResolveGlobalPath returns the path to the user-wide pools.toml (may not exist).
func ResolveGlobalPath(cfg config.Config, homeDir, configDir string) string {
	if f := strings.TrimSpace(cfg.Pools.File); f != "" {
		return expandHome(f, homeDir)
	}
	if strings.TrimSpace(configDir) == "" {
		return ""
	}
	return filepath.Join(configDir, config.DefaultPoolsFileName)
}

// LoadGlobal reads pools.toml from the configured global path.
// Returns (nil, nil) when the file does not exist.
func LoadGlobal(cfg config.Config, homeDir, configDir string) ([]Def, error) {
	path := ResolveGlobalPath(cfg, homeDir, configDir)
	if path == "" {
		return nil, nil
	}
	mf, err := LoadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("load pools %q: %w", path, err)
	}
	if mf == nil {
		return nil, nil
	}
	return mf.Pools, nil
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
