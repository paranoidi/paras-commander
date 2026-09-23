package panel

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// GroupSelectMeta carries optional meta-column data for group select/unselect matching.
type GroupSelectMeta struct {
	Cols     []map[string]string // per-column abs-path → value maps (nil = no meta matching)
	OnlyMeta bool                // when true, skip filename matching; match only via Cols
}

// Match reports whether e matches under matcher, honoring m's meta-column data: the basename is
// checked first unless OnlyMeta is set, then any non-empty column value at e.Path. Shared by
// group select/unselect matching (groupMatchedPaths) and the Filter dialog's meta-aware pattern
// filter/preview.
func (m GroupSelectMeta) Match(matcher GroupMatcher, e localfs.Entry) bool {
	if !m.OnlyMeta && matcher.Match(e.Name) {
		return true
	}
	for _, col := range m.Cols {
		if v, ok := col[e.Path]; ok && v != "" && matcher.Match(v) {
			return true
		}
	}
	return false
}

// GroupPatternMode selects shell glob, regexp, or simple substring matching for group select.
type GroupPatternMode int

const (
	GroupPatternShell GroupPatternMode = iota
	GroupPatternRegex
	GroupPatternSimple
)

// GroupMatcher compiles a group-select pattern once and matches many basenames.
type GroupMatcher struct {
	mode          GroupPatternMode
	pattern       string
	caseSensitive bool
	rx            *regexp.Regexp
}

// NewGroupMatcher builds a matcher for the given pattern and options.
func NewGroupMatcher(pattern string, mode GroupPatternMode, caseSensitive bool) (GroupMatcher, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return GroupMatcher{}, fmt.Errorf("pattern is empty")
	}
	m := GroupMatcher{
		mode:          mode,
		pattern:       pattern,
		caseSensitive: caseSensitive,
	}
	if !caseSensitive && mode != GroupPatternRegex {
		m.pattern = strings.ToLower(pattern)
	}
	switch mode {
	case GroupPatternShell:
		if _, err := filepath.Match(pattern, "x"); err != nil {
			return GroupMatcher{}, fmt.Errorf("invalid shell pattern: %w", err)
		}
	case GroupPatternRegex:
		re, err := regexp.Compile(pattern)
		if err != nil {
			return GroupMatcher{}, fmt.Errorf("invalid regexp: %w", err)
		}
		m.rx = re
	case GroupPatternSimple:
	default:
		return GroupMatcher{}, fmt.Errorf("unknown group pattern mode %d", mode)
	}
	return m, nil
}

// Match reports whether name matches the compiled pattern.
func (m GroupMatcher) Match(name string) bool {
	switch m.mode {
	case GroupPatternShell:
		value := name
		if !m.caseSensitive {
			value = strings.ToLower(value)
		}
		matched, _ := filepath.Match(m.pattern, value)
		return matched
	case GroupPatternRegex:
		return m.rx.MatchString(name)
	case GroupPatternSimple:
		n := name
		if !m.caseSensitive {
			n = strings.ToLower(n)
		}
		return strings.Contains(n, m.pattern)
	default:
		return false
	}
}
