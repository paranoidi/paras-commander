package panel

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

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
		pattern := m.pattern
		value := name
		if !m.caseSensitive {
			pattern = strings.ToLower(pattern)
			value = strings.ToLower(value)
		}
		matched, _ := filepath.Match(pattern, value)
		return matched
	case GroupPatternRegex:
		return m.rx.MatchString(name)
	case GroupPatternSimple:
		n := name
		p := m.pattern
		if !m.caseSensitive {
			n = strings.ToLower(n)
			p = strings.ToLower(p)
		}
		return strings.Contains(n, p)
	default:
		return false
	}
}
