package usermenu

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MenuStubTOML is written when no menu.toml exists yet. All entries are commented
// so the user can uncomment and customize.
const MenuStubTOML = `# F2 user menu — see docs/config.md
#
# Each action is a [[entry]] table. Keep the table name "entry" for every
# action (e.g. [[toolname]] is ignored — only [[entry]] is loaded).
# Press the entry key in the F2 menu to run the command immediately.
#
# [[entry]]
# key = "p"
# title = "Print working directory"
# command = "pwd"
# default = true
#
# [[entry]]
# key = "e"
# title = "Echo active panel directory"
# command = "echo %d"
#
# Options:
#
# when = optional visibility (e.g. f *.go)
# shell_patterns = true   # true/1 = glob in when=; false/0 = regex
# interactive = true      # suspend TUI, attach terminal (lazygit, vim, htop)
# detach = true           # start in background (xdg-open, GUI helpers)
# interactive and detach cannot both be set on one entry
#
# Macros (expanded in command before run):
#
# %%  literal % character
# %f  basename of the highlighted file on the active panel
# %F  basename of the highlighted file on the other panel
# %d  directory path of the active panel
# %D  directory path of the other panel
# %t  quoted paths of tagged files in the active panel's current directory (space-separated)
# %T  quoted paths of tagged files in the other panel's current directory (space-separated)
`

// WriteMenuStub creates path with MenuStubTOML when the file does not exist yet.
func WriteMenuStub(path string) (created bool, err error) {
	if strings.TrimSpace(path) == "" {
		return false, fmt.Errorf("menu stub path is required")
	}
	path = filepath.Clean(path)
	if _, statErr := os.Stat(path); statErr == nil {
		return false, nil
	} else if !os.IsNotExist(statErr) {
		return false, fmt.Errorf("stat menu stub %q: %w", path, statErr)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create menu stub dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(MenuStubTOML), 0o644); err != nil {
		return false, fmt.Errorf("write menu stub %q: %w", path, err)
	}
	return true, nil
}
