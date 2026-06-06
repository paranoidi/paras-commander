package usermenu

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/configdoc"
)

// MenuStubTOML is written when no menu.toml exists yet. All entries are commented
// so the user can uncomment and customize.
const MenuStubTOML = `# F2 user menu
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
# ── File-level ─────────────────────────────────────────────────────────────
#
# shell_patterns  bool   default: true
#   Default pattern mode for when= on entries that omit shell_patterns.
#   true  → glob (bare patterns match basename)
#   false → regex
#
# ── [[entry]] fields ───────────────────────────────────────────────────────
#
# title           string   required
# command         string   required
#
# key             string   optional   (single letter)
#   Pin Alt+letter shortcut; otherwise derived from title.
#
# when            string | [string]   optional   default: always visible
#   Visibility filter; OR semantics across list items.
#
# shell_patterns  bool     optional   default: file-level (else true)
#   Override file default for this entry's when= patterns.
#   true → glob; false → regex.
#
# default         bool     optional   default: false
#   Highlight this row among visible entries.
#
# run_for_each    [string] optional   values: "files" | "dirs"
#   Run once per selected item (or cursor when nothing selected).
#   command must include %f; not combinable with interactive or detach.
#
# shell           bool     optional   default: false
#   Force sh -c even without shell operators (>> | && …).
#
# interactive     bool     optional   default: false
#   Suspend TUI and attach terminal (vim, lazygit, htop).
#   Mutually exclusive with detach, background, and run_for_each.
#
# detach          bool     optional   default: false
#   Start in background (xdg-open, GUI helpers).
#
# background      bool     optional   default: false
#   Capture output without Commands view; notify on failure/stderr;
#   refresh panel when done.
#
# pool            string   optional
#   Work pool name from pools.toml [[pools]]; default/background mode only.
#   Omit for unlimited parallelism; not combinable with interactive or detach.
#
# Macros (substituted before the command line is parsed into argv):
#
#   Do not wrap macros in quotes — the app quotes path/name values when expanding.
#   Good:  command = "gzip -9 %f"
#   Bad:   command = "gzip -9 \"%f\""  or  'gzip -9 "%f"'
#
#     %%  literal % character
#     %f  basename of the highlighted file on the active panel (run_for_each: iterated absolute path)
#     %F  basename of the highlighted file on the other panel
#     %d  directory path of the active panel
#     %D  directory path of the other panel
#     %t  tagged files in the active panel's current directory (expanded as quoted tokens)
#     %T  tagged files in the other panel's current directory (expanded as quoted tokens)
#
# --- end of documentation ---
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

// RefreshDocumentation replaces the leading documentation block in path with
// MenuStubTOML while preserving user [[entry]] configuration.
func RefreshDocumentation(path string) (bool, error) {
	return configdoc.RefreshDocumentation(path, MenuStubTOML)
}
