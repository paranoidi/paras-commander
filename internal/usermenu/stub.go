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
const MenuStubTOML = `# User function menu
#
# Each action is a uniquely-named TOML table, e.g. [pwd] or [tools.disk_use].
# The table name is just an identifier (TOML itself enforces uniqueness
# within its parent); it never appears in the UI — only title= does.
# Display order in the function menu is the order table headers appear in
# this file, not alphabetical.
# Press the highlighted letter in the function menu to run an entry immediately
# (no Alt needed). Esc closes the menu (or, inside a submenu, steps back one level).
#
# [pwd]
# title = "Print working directory"
# command = "pwd"
# default = true
#
# [echo_dir]
# title = "Echo active panel directory"
# command = "echo %d"
#
# A table becomes a submenu once it has its own nested [parent.child] tables
# instead of a command — submenus can nest arbitrarily deep. A submenu table
# cannot also set command / run_for_each / pool / toast / interactive /
# detach / background / dialog (mutually exclusive with being a container).
# key=/default= uniqueness is scoped per menu level, so a key can be reused
# across sibling submenus but not twice within the same level.
#
# [tools]
# title = "Tools"
# key = "t"
#
# [tools.disk_use]
# title = "Show disk usage"
# command = "du -sh %f"
#
# [tools.format]
# title = "Format code"
# command = "gofmt -w %f"
#
# ── File-level ─────────────────────────────────────────────────────────────
#
# shell_patterns  bool   default: true
#   Default pattern mode for when= on entries that omit shell_patterns.
#   true  → glob (bare patterns match basename)
#   false → regex
#
# ── entry table fields ─────────────────────────────────────────────────────
#
# title           string   required
# command         string   required (omit only when this table is a submenu container)
#
# toast           string   optional
#   Message shown in the status bar after the command succeeds.
#   Suppressed on error. For detach entries it replaces "Started …".
#
# key             string   optional   (single letter)
#   Pin the function-menu activation letter; otherwise derived from title.
#   No letters are reserved. Must be unique among siblings at the same menu
#   level (the same letter can be reused across different submenus).
#
# when            string | [string]   optional   default: always visible
#   Visibility filter; OR semantics across list items. Also applies to a
#   submenu table itself — a submenu with zero visible children is hidden.
#
# shell_patterns  bool     optional   default: file-level (else true)
#   Override file default for this entry's when= patterns. A submenu's own
#   resolved value cascades down as the default for its children.
#   true → glob; false → regex.
#
# default         bool     optional   default: false
#   Highlight this row among visible entries at the same menu level.
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
# dialog          bool     optional   default: false
#   Show command stdout in a modal dialog when the command finishes.
#   Mutually exclusive with interactive, detach, background, and run_for_each.
#
# dialog_width    string | int   optional   default: 80% of terminal width
#   Dialog width: "80%" for 80 % of terminal width, or "100" for 100 columns.
#   Bare integer also accepted: 100 is equivalent to "100".
#
# dialog_height   string | int   optional   default: 60% of terminal height
#   Dialog height: "50%" for 50 % of terminal height, or "20" for 20 rows.
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
