package panel

import (
	"regexp"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// PatternMetaFilterID is the EntryFilter.ID used by PatternFilter when meta is non-nil, so the
// meta handler knows which panels' active filters to re-run as meta results arrive
// (internal/apphandler/meta.Handler.HandleRenderFlush).
const PatternMetaFilterID = "pattern-meta"

// FilterSpec describes the panel Filter dialog's result: an optional pattern, an optional
// file-type regexp (OR of the checked types, matched against the basename only), and the
// files/dirs/invert switches. Match = types AND pattern; Invert negates that, and
// FilesOnly/DirsOnly apply outside the inversion.
type FilterSpec struct {
	Pattern       string
	Mode          GroupPatternMode
	CaseSensitive bool
	FilesOnly     bool
	DirsOnly      bool
	Invert        bool
	Types         *regexp.Regexp // nil = no type restriction
	TypeLabel     string         // checked type names for the border label
}

// Empty reports whether the spec restricts nothing (no pattern and no types).
func (f FilterSpec) Empty() bool { return f.Pattern == "" && f.Types == nil }

// Label is the panel border label for the spec.
func (f FilterSpec) Label() string {
	parts := []string{}
	if f.Invert {
		parts = append(parts, "not")
	}
	if f.Pattern != "" {
		parts = append(parts, f.Pattern)
	}
	if f.TypeLabel != "" {
		parts = append(parts, f.TypeLabel)
	}
	return "Filter: " + strings.Join(parts, " ")
}

// matchFunc compiles the pattern once and returns the per-entry predicate (files/dirs-only,
// types, pattern, invert). meta may be the zero value.
func (f FilterSpec) matchFunc() (func(e localfs.Entry, meta GroupSelectMeta) bool, error) {
	var matcher GroupMatcher
	if f.Pattern != "" {
		var err error
		if matcher, err = NewGroupMatcher(f.Pattern, f.Mode, f.CaseSensitive); err != nil {
			return nil, err
		}
	}
	return func(e localfs.Entry, meta GroupSelectMeta) bool {
		if (f.DirsOnly && !e.IsDir()) || (f.FilesOnly && e.IsDir()) {
			return false
		}
		m := (f.Types == nil || f.Types.MatchString(e.Name)) &&
			(f.Pattern == "" || meta.Match(matcher, e))
		return m != f.Invert
	}, nil
}

// PatternFilter builds an EntryFilter from spec. Directories always stay visible unless
// FilesOnly/DirsOnly is set. meta is nil when there are no meta columns to match against;
// otherwise it is called on every filter rebuild (RefreshEntryFilter) to read the current live
// GroupSelectMeta — a closure rather than a snapshot because meta results arrive asynchronously
// and the underlying maps are replaced wholesale on a directory change. Returns an error when
// the pattern fails to compile.
func PatternFilter(spec FilterSpec, meta func() GroupSelectMeta) (*EntryFilter, error) {
	match, err := spec.matchFunc()
	if err != nil {
		return nil, err
	}
	id := "pattern"
	if meta != nil {
		id = PatternMetaFilterID
	}
	return &EntryFilter{
		ID:          id,
		FiltersDirs: spec.DirsOnly || spec.FilesOnly,
		Label:       spec.Label(),
		Match: func(e localfs.Entry, _ *State) bool {
			var m GroupSelectMeta
			if meta != nil {
				m = meta()
			}
			return match(e, m)
		},
	}, nil
}
