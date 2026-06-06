package metacmds

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MetaStubTOML is written when no meta.toml exists yet. All entries are commented
// so the user can uncomment and customize.
const MetaStubTOML = `# meta.toml — Meta column commands
#
# Each command is a [[entry]] block. Shell scripts use %f for the entry absolute path.
# Commands are run via: sh -c '<expanded script>'
#
# If output contains \t or \n the output will be split and aligned.
#
# ── File-level ─────────────────────────────────────────────────────────────
#
# shell_patterns  bool   default: true
#   Default pattern mode for when= on entries that omit shell_patterns.
#   true  → glob (bare patterns match basename, e.g. "*.py")
#   false → regex
#
# ── [[entry]] fields ───────────────────────────────────────────────────────
#
# name            string   required
#   Unique id shown in the meta picker.
#
# description     string   required
#   Human-readable label in the picker.
#
# file            string   optional   (at least one of file | dirs required)
#   Shell script for regular files. %f = absolute path.
#
# dirs            string   optional
#   Shell script for directories. %f = absolute path.
#
# when            string | [string]   optional   default: always visible
#   Visibility filter; OR semantics across list items.
#
# shell_patterns  bool     optional   default: file-level (else true)
#   Override file default for this entry's when= patterns.
#   true → glob; false → regex.
#
# extensions      [string] optional   default: run for every entry
#   Glob filter on basename (e.g. ["*.py", "*.go"]). Only entries whose basename
#   matches at least one pattern get processed. Empty or omitted means no filter.
#
# cache           bool     optional   default: false
#   Session-scoped in-memory cache keyed by absolute path (never on disk).
#
# workers         int      optional   default: meta.default_entry_workers (2)
#   Number of concurrent background goroutines for this entry. Clamped to 64.
#
# Do not wrap %f in quotes in the script — the app quotes the path when expanding.
#
# Examples:
#
# [[entry]]
# name = "count-items"
# description = "Count files and folders"
# dirs = """
#   f=$(find %f -maxdepth 1 -mindepth 1 -type f | wc -l | awk '{print $1}')
#   d=$(find %f -maxdepth 1 -mindepth 1 -type d | wc -l | awk '{print $1}')
#   printf '\t%s\t\t%s\n' "$f" "$d"
# """
#
# [[entry]]
# name = "mkvinfo"
# description = "MKV Info (length, resolution)"
# when = ["*.mkv", "*.MKV"]
# cache = true
# file = """
#   mkvinfo %f 2>/dev/null | awk '
#     !/Default/&&/Duration/{split($4,d,":");dur=d[1]":"d[2]}
#     /Pixel/{if(/width/)w=$NF;else h=$NF}
#     END{print dur"\t"w"x"h}'
# """
#
# [[entry]]
# name = "line-count"
# description = "Line count (text files)"
# when = ["*.py", "*.go", "*.js", "*.ts", "*.rs", "*.c", "*.h", "*.cpp"]
# cache = true
# file = "wc -l < %f | tr -d ' '"
`

// WriteMetaStub creates path with MetaStubTOML when the file does not exist yet.
func WriteMetaStub(path string) (created bool, err error) {
	if strings.TrimSpace(path) == "" {
		return false, fmt.Errorf("meta stub path is required")
	}
	path = filepath.Clean(path)
	if _, statErr := os.Stat(path); statErr == nil {
		return false, nil
	} else if !os.IsNotExist(statErr) {
		return false, fmt.Errorf("stat meta stub %q: %w", path, statErr)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create meta stub dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(MetaStubTOML), 0o644); err != nil {
		return false, fmt.Errorf("write meta stub %q: %w", path, err)
	}
	return true, nil
}
