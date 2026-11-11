package theme

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

var dialogSurfaceForegroundStyleKeys = map[string]struct{}{
	"dialog.option.inactive":         {},
	"dialog.option.active":           {},
	"dialog.option.active.selected":  {},
	"dialog.option.selected":         {},
	"dialog.option.invalid":          {},
	"dialog.option.disabled":         {},
	"dialog.status.selection_size":   {},
	"dialog.progress.label.on_fill":  {},
	"dialog.progress.label.on_track": {},
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
		spec := `{ fg = "fg", bg = "bg" }`
		if _, ok := dialogSurfaceForegroundStyleKeys[key]; ok {
			spec = `{ fg = "fg" }`
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
		if skip != nil && skip[key] || requiredSet[key] || isColorOverride(key) {
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
	colors := map[string]string{"black": `"#040506"`, "white": `"#010203"`, "yellow": `"#ffff00"`}
	for key, value := range overrides {
		if name, ok := strings.CutPrefix(key, "palette."); ok {
			colors[name] = value
		}
	}
	writeSortedTable(&builder, colors)
	builder.WriteString("\n[roles]\n")
	roles := map[string]string{}
	for key, value := range overrides {
		if name, ok := strings.CutPrefix(key, "roles."); ok {
			roles[name] = value
		}
	}
	writeSortedTable(&builder, roles)
	builder.WriteString("\n")
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

// isColorOverride reports an override key that sets a [palette] ("palette.NAME") or [roles]
// ("roles.NAME") entry (the value is raw TOML) instead of a style.
func isColorOverride(key string) bool {
	return strings.HasPrefix(key, "palette.") || strings.HasPrefix(key, "roles.")
}

func writeSortedTable(b *strings.Builder, table map[string]string) {
	keys := make([]string, 0, len(table))
	for k := range table {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(b, "%s = %s\n", k, table[k])
	}
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
