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
# Press Alt+the highlighted letter in the F2 menu to run an entry immediately.
#
# [[entry]]
# title = "Print working directory"
# command = "pwd"
# default = true
#
# [[entry]]
# title = "Echo active panel directory"
# command = "echo %d"
#
# Options:
# 
# key = "p"
# 	  pin a shortcut letter (otherwise assigned from the title, like the meta picker)
# when = "*.go"
# 	  optional visibility (e.g. *.go)
# shell_patterns = true   
#     true = glob patterns in when=; 
#     false = regex patterns in when=
# interactive = true
#     suspend TUI, attach terminal (lazygit, vim, htop)
# detach = true           
#     start in background (xdg-open, GUI helpers)
# background = true       
#     capture output without opening Commands view; notify on failure/stderr; 
#     refresh panel when done
#     interactive, detach, and background cannot be combined on one entry
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
