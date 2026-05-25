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
# Each command is a [[entry]] block. The entry path is passed as $1 to the shell command.
# Commands are run via: sh -c '<command>' sh '<path>'
#
# Fields:
#   name        = unique identifier shown in the meta picker dialog (required)
#   description = human-readable description shown in the picker (required)
#   file        = shell command run for regular files ($1 = absolute path)
#   dirs        = shell command run for directories ($1 = absolute path)
#   extensions  = optional list of glob patterns; command only runs for entries whose
#                 basename matches at least one pattern (e.g. ["*.py", "*.go"]).
#                 Empty or omitted means run for every entry.
#   cache       = when true, results are stored in a session-scoped in-memory cache keyed
#                 by absolute path. Re-entering a directory reuses cached values instead of
#                 re-running the command. The cache lasts for the lifetime of the application
#                 and is never written to disk. Default: false.
#
# At least one of file or dirs is required per entry. Both may be set.
#
# Examples:
#
# [[entry]]
# name = "count-items"
# description = "Count files and folders"
# dirs = """
#   f=$(find "$1" -maxdepth 1 -mindepth 1 -type f | wc -l | awk '{print $1}')
#   d=$(find "$1" -maxdepth 1 -mindepth 1 -type d | wc -l | awk '{print $1}')
#   printf '\t%s\t\t%s\n' "$f" "$d"
# """
#
# [[entry]]
# name = "mkvinfo"
# description = "MKV Info (length, resolution)"
# extensions = ["*.mkv", "*.MKV"]
# cache = true
# file = """
#   mkvinfo "$1" 2>/dev/null | awk '
#     !/Default/&&/Duration/{split($4,d,":");dur=d[1]":"d[2]}
#     /Pixel/{if(/width/)w=$NF;else h=$NF}
#     END{print dur"\t"w"x"h}'
# """
#
# [[entry]]
# name = "line-count"
# description = "Line count (text files)"
# extensions = ["*.py", "*.go", "*.js", "*.ts", "*.rs", "*.c", "*.h", "*.cpp"]
# cache = true
# file = "wc -l < \"$1\" | tr -d ' '"
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
