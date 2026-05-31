package theme

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

var dialogOptionStyleKeys = map[string]struct{}{
	"dialog.option.inactive":        {},
	"dialog.option.active":          {},
	"dialog.option.active.selected": {},
	"dialog.option.selected":        {},
	"dialog.option.invalid":         {},
}

// TestThemeBytes builds a minimal valid theme TOML from requiredStyleKeys plus optional overrides.
func TestThemeBytes(t *testing.T, overrides map[string]string) []byte {
	t.Helper()
	return buildTestThemeBytes("test-theme", nil, overrides)
}

// TestThemeBytesNamed is like TestThemeBytes but allows a custom theme name and skipped keys.
func TestThemeBytesNamed(t *testing.T, name string, skip map[string]bool, overrides map[string]string) []byte {
	t.Helper()
	return buildTestThemeBytes(name, skip, overrides)
}

func buildTestThemeBytes(name string, skip map[string]bool, overrides map[string]string) []byte {
	bySection := map[string]map[string]string{}
	for _, key := range requiredStyleKeys {
		if skip != nil && skip[key] {
			continue
		}
		section, relative := styleSectionRelative(key)
		spec := `{ fg = "white", bg = "black" }`
		if _, ok := dialogOptionStyleKeys[key]; ok {
			spec = `{ fg = "white" }`
		}
		if override, ok := overrides[key]; ok {
			spec = override
		}
		if bySection[section] == nil {
			bySection[section] = map[string]string{}
		}
		bySection[section][relative] = spec
	}
	requiredSet := makeStyleKeySet(requiredStyleKeys)
	for key, spec := range overrides {
		if skip != nil && skip[key] || requiredSet[key] {
			continue
		}
		section, relative := styleSectionRelative(key)
		if bySection[section] == nil {
			bySection[section] = map[string]string{}
		}
		bySection[section][relative] = spec
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "name = %q\n\n", name)
	builder.WriteString("[palette]\n")
	builder.WriteString("black = \"#040506\"\n")
	builder.WriteString("white = \"#010203\"\n")
	builder.WriteString("yellow = \"#ffff00\"\n\n")
	for _, root := range styleSectionRoots {
		entries, ok := bySection[root]
		if !ok || len(entries) == 0 {
			continue
		}
		fmt.Fprintf(&builder, "[%s]\n", root)
		keys := make([]string, 0, len(entries))
		for k := range entries {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&builder, "%s = %s\n", k, entries[k])
		}
		builder.WriteString("\n")
	}
	return []byte(builder.String())
}

func styleSectionRelative(fullKey string) (section, relative string) {
	for _, root := range styleSectionRoots {
		prefix := root + "."
		if strings.HasPrefix(fullKey, prefix) {
			return root, strings.TrimPrefix(fullKey, prefix)
		}
	}
	return "", fullKey
}
