package panel

import "github.com/paranoidi/paras-commander/internal/localfs"

// PatternMetaFilterID is the EntryFilter.ID used by PatternFilter when meta is non-nil, so the
// meta handler knows which panels' active filters to re-run as meta results arrive
// (internal/apphandler/meta.Handler.HandleRenderFlush).
const PatternMetaFilterID = "pattern-meta"

// PatternFilter builds an EntryFilter that narrows visible entries to those whose basename (or,
// when meta is non-nil, a meta column value) matches pattern under mode (shell glob / regexp /
// simple substring). Directories always stay visible unless dirsOnly is set, in which case
// directories are filtered by pattern and files are hidden. meta is nil when
// there are no meta columns to match against; otherwise it is called on every filter rebuild
// (RefreshEntryFilter) to read the current live GroupSelectMeta — a closure rather than a
// snapshot because meta results arrive asynchronously and the underlying maps are replaced
// wholesale on a directory change. Returns an error when pattern fails to compile.
func PatternFilter(pattern string, mode GroupPatternMode, caseSensitive, dirsOnly bool, meta func() GroupSelectMeta) (*EntryFilter, error) {
	matcher, err := NewGroupMatcher(pattern, mode, caseSensitive)
	if err != nil {
		return nil, err
	}
	id := "pattern"
	if meta != nil {
		id = PatternMetaFilterID
	}
	return &EntryFilter{
		ID:          id,
		FiltersDirs: dirsOnly,
		Label:       "Filter: " + pattern,
		Match: func(e localfs.Entry, _ *State) bool {
			if dirsOnly && !e.IsDir() {
				return false
			}
			if meta != nil {
				return meta().Match(matcher, e)
			}
			return matcher.Match(e.Name)
		},
	}, nil
}
