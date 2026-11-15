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

// PatternScope is which kind of entry a Filter / Select group pattern applies to. The zero value
// is ScopeFiles, the dialogs' default.
type PatternScope int

const (
	// ScopeFiles matches files only. As a panel filter it leaves every directory visible, so the
	// user keeps navigating while the files are narrowed.
	ScopeFiles PatternScope = iota
	// ScopeDirs matches directories only. As a panel filter it hides every file.
	ScopeDirs
	// ScopeBoth matches files and directories alike.
	ScopeBoth
)

// Targets reports whether the scope applies to an entry of that kind.
func (s PatternScope) Targets(isDir bool) bool {
	switch s {
	case ScopeFiles:
		return !isDir
	case ScopeDirs:
		return isDir
	default:
		return true
	}
}

// FilterSpec describes the Filter / Select group dialog's result: an optional pattern, an
// optional file-type regexp (OR of the checked types, matched against the basename only), the
// scope and invert switch. Match = types AND pattern; Invert negates that, and only entries the
// Scope targets can match.
type FilterSpec struct {
	Pattern       string
	Mode          GroupPatternMode
	CaseSensitive bool
	Scope         PatternScope
	Invert        bool
	Types         *regexp.Regexp // nil = no type restriction
	TypeLabel     string         // checked type names for the border label
}

// Empty reports whether the spec restricts nothing (no pattern and no types).
func (f FilterSpec) Empty() bool { return f.Pattern == "" && f.Types == nil }

// Label is the panel border label for the spec. The scope is named only when it is not the
// default (ScopeFiles).
func (f FilterSpec) Label() string {
	parts := []string{}
	switch f.Scope {
	case ScopeDirs:
		parts = append(parts, "dirs")
	case ScopeBoth:
		parts = append(parts, "all")
	}
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

// MatchFunc compiles the pattern once and returns the per-entry match predicate (scope, types,
// pattern, invert): what Select group selects and the dialogs' live preview counts. An entry
// outside the scope never matches. meta may be the zero value.
func (f FilterSpec) MatchFunc() (func(e localfs.Entry, meta GroupSelectMeta) bool, error) {
	var matcher GroupMatcher
	if f.Pattern != "" {
		var err error
		if matcher, err = NewGroupMatcher(f.Pattern, f.Mode, f.CaseSensitive); err != nil {
			return nil, err
		}
	}
	return func(e localfs.Entry, meta GroupSelectMeta) bool {
		if !f.Scope.Targets(e.IsDir()) {
			return false
		}
		m := (f.Types == nil || f.Types.MatchString(e.Name)) &&
			(f.Pattern == "" || meta.Match(matcher, e))
		return m != f.Invert
	}, nil
}

// VisibleFunc is MatchFunc as a panel filter's visibility predicate: ".." always shows, and an
// entry outside the scope shows under ScopeFiles (directories stay navigable) and hides under
// ScopeDirs (files).
func (f FilterSpec) VisibleFunc() (func(e localfs.Entry, meta GroupSelectMeta) bool, error) {
	match, err := f.MatchFunc()
	if err != nil {
		return nil, err
	}
	return func(e localfs.Entry, meta GroupSelectMeta) bool {
		if e.Name == ".." {
			return true
		}
		if !f.Scope.Targets(e.IsDir()) {
			return f.Scope == ScopeFiles
		}
		return match(e, meta)
	}, nil
}

// PatternFilter builds an EntryFilter from spec (VisibleFunc). meta is
// nil when there are no meta columns to match against; otherwise it is called on every filter rebuild (RefreshEntryFilter) to read the current live
// GroupSelectMeta — a closure rather than a snapshot because meta results arrive asynchronously
// and the underlying maps are replaced wholesale on a directory change. Returns an error when
// the pattern fails to compile.
func PatternFilter(spec FilterSpec, meta func() GroupSelectMeta) (*EntryFilter, error) {
	match, err := spec.VisibleFunc()
	if err != nil {
		return nil, err
	}
	id := "pattern"
	if meta != nil {
		id = PatternMetaFilterID
	}
	return &EntryFilter{
		ID:    id,
		Label: spec.Label(),
		Match: func(e localfs.Entry, _ *State) bool {
			var m GroupSelectMeta
			if meta != nil {
				m = meta()
			}
			return match(e, m)
		},
	}, nil
}
