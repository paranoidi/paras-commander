package panel

import "github.com/paranoidi/paras-commander/internal/localfs"

// PatternFilter builds an EntryFilter that narrows visible entries to those whose basename
// matches pattern under mode (shell glob / regexp / simple substring), optionally restricted to
// files only or directories only. Returns an error when pattern fails to compile.
func PatternFilter(pattern string, mode GroupPatternMode, caseSensitive, filesOnly, dirsOnly bool) (*EntryFilter, error) {
	matcher, err := NewGroupMatcher(pattern, mode, caseSensitive)
	if err != nil {
		return nil, err
	}
	return &EntryFilter{
		ID:    "pattern",
		Label: "Filter: " + pattern,
		Match: func(e localfs.Entry, _ *State) bool {
			if filesOnly && e.IsDir() {
				return false
			}
			if dirsOnly && !e.IsDir() {
				return false
			}
			return matcher.Match(e.Name)
		},
	}, nil
}
