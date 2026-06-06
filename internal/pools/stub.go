package pools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/configdoc"
)

// PoolsStubTOML is written when no pools.toml exists yet. All entries are commented
// so the user can uncomment and customize.
const PoolsStubTOML = `# pools.toml — work pool concurrency limits
#
# User menu entries may set pool = "name" to limit parallel runs.
# Omit pool on an entry for unlimited parallelism.
#
# [[pools]]
# name = "cpu"
# max_parallel = 8
#
# [[pools]]
# name = "io"
# max_parallel = 2
#
# --- end of documentation ---
`

// WritePoolsStub creates path with PoolsStubTOML when the file does not exist yet.
func WritePoolsStub(path string) (created bool, err error) {
	if strings.TrimSpace(path) == "" {
		return false, fmt.Errorf("pools stub path is required")
	}
	path = filepath.Clean(path)
	if _, statErr := os.Stat(path); statErr == nil {
		return false, nil
	} else if !os.IsNotExist(statErr) {
		return false, fmt.Errorf("stat pools stub %q: %w", path, statErr)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create pools stub dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(PoolsStubTOML), 0o644); err != nil {
		return false, fmt.Errorf("write pools stub %q: %w", path, err)
	}
	return true, nil
}

// RefreshDocumentation replaces the leading documentation block in path with
// PoolsStubTOML while preserving user [[pools]] configuration.
func RefreshDocumentation(path string) (bool, error) {
	return configdoc.RefreshDocumentation(path, PoolsStubTOML)
}
