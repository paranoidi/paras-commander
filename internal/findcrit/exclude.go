package findcrit

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Excludes are shell patterns whose matches the find walk neither indexes nor descends into. A
// pattern with a '/' matches the path relative to the find root, others the basename.
type Excludes []string

// ParseExcludes splits an Exclude field on commas and whitespace and validates each pattern.
func ParseExcludes(s string) (Excludes, error) {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	if len(fields) == 0 {
		return nil, nil
	}
	out := make(Excludes, 0, len(fields))
	for _, f := range fields {
		f = strings.Trim(f, "/")
		if f == "" {
			continue
		}
		if _, err := filepath.Match(f, "x"); err != nil {
			return nil, fmt.Errorf("exclude %q: %w", f, err)
		}
		out = append(out, f)
	}
	return out, nil
}

// Match reports whether the entry at rel (relative to the find root, '/'-separated) is excluded.
func (e Excludes) Match(rel string) bool {
	if len(e) == 0 {
		return false
	}
	base := rel[strings.LastIndexByte(rel, '/')+1:]
	for _, p := range e {
		value := base
		if strings.Contains(p, "/") {
			value = rel
		}
		if ok, _ := filepath.Match(p, value); ok {
			return true
		}
	}
	return false
}
