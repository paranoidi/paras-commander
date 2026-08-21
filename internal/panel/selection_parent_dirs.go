package panel

import (
	"fmt"
	"sort"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// ParentDirsOf returns the unique immediate parent directory of each path
// (sorted, deduplicated). Uses pathloc so sftp:// paths resolve correctly.
// Paths that fail to parse, or are already at the root, are skipped.
func ParentDirsOf(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, raw := range paths {
		loc, err := pathloc.Parse(raw)
		if err != nil {
			continue
		}
		parent := loc.Parent()
		if parent.Equal(loc) {
			continue
		}
		s := parent.String()
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// SelectedParentDirs returns ParentDirsOf applied to every currently-selected
// path on this panel (the whole SelectedPaths set, not just what's listed).
func (s *State) SelectedParentDirs() []string {
	paths := make([]string, 0, len(s.SelectedPaths))
	for p := range s.SelectedPaths {
		paths = append(paths, p)
	}
	return s.ParentDirsExcludingSelf(paths)
}

// ParentDirsExcludingSelf is ParentDirsOf(paths) with this panel's own current
// directory dropped from the result. Selecting the directory a panel is
// currently browsing, from within itself, doesn't correspond to any listed
// entry — it isn't a child of itself — so callers that apply the result via
// BulkAddSelections on this panel must go through this rather than ParentDirsOf
// directly.
func (s *State) ParentDirsExcludingSelf(paths []string) []string {
	dirs := ParentDirsOf(paths)
	cur := cleanPathString(s.Path.String())
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if cleanPathString(d) == cur {
			continue
		}
		out = append(out, d)
	}
	return out
}

// SelectDirs adds dirs (already known to be directories, e.g. from ParentDirsOf /
// ParentDirsExcludingSelf) to this panel's selection via BulkAddSelections, and
// formats the human-readable result: conflicts reports whether any prior
// conflicting selections were removed, and message is the pluralized "Selected N
// director{y,ies}" success text callers show when conflicts is false. dirs must
// be non-empty.
func (s *State) SelectDirs(dirs []string) (message string, conflicts bool) {
	conflicts = s.BulkAddSelections(dirs, func(string) bool { return true })
	word := "directories"
	if len(dirs) == 1 {
		word = "directory"
	}
	return fmt.Sprintf("Selected %d %s", len(dirs), word), conflicts
}
