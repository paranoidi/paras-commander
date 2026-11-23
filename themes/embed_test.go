package themes

import (
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestDefaultThemeUsesOnly16ANSIColors keeps default.toml on the terminal's 16 ANSI colors: every
// [palette] value is an index 0–15 (or "default"), every [roles] value is a [palette] name and no
// style names a hex color directly.
func TestDefaultThemeUsesOnly16ANSIColors(t *testing.T) {
	data, err := Files.ReadFile("default.toml")
	if err != nil {
		t.Fatal(err)
	}
	section := ""
	paletteNames := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "[") {
			section = t
		} else if m := regexp.MustCompile(`^([a-z_]+)\s*=`).FindStringSubmatch(t); m != nil && section == "[palette]" {
			paletteNames[m[1]] = true
		}
	}
	section = ""
	entry := regexp.MustCompile(`^([a-z_]+)\s*=\s*([^#]+?)\s*(#.*)?$`)
	hexColor := regexp.MustCompile(`"#[0-9a-fA-F]{6}"`)
	for i, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			section = trimmed
			continue
		}
		if hexColor.MatchString(trimmed) && !strings.HasPrefix(trimmed, "#") {
			t.Errorf("line %d: hex color in default theme: %s", i+1, trimmed)
		}
		if section != "[palette]" && section != "[roles]" {
			continue
		}
		m := entry.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		if section == "[roles]" {
			if name := strings.Trim(m[2], `"`); !paletteNames[name] && name != "default" {
				t.Errorf("line %d: roles.%s = %s is not a [palette] name", i+1, m[1], m[2])
			}
			continue
		}
		if m[2] == `"default"` {
			continue
		}
		if n, err := strconv.Atoi(m[2]); err != nil || n < 0 || n > 15 {
			t.Errorf("line %d: palette.%s = %s is not an ANSI index 0-15", i+1, m[1], m[2])
		}
	}
}

// inlineTableExtensions lists the keys of a theme file that are assigned an inline table and also
// extended by a dotted key (`header = {...}` plus `header.carousel = {...}`): invalid TOML. The
// parent is written `header.base = {...}` instead.
func inlineTableExtensions(text string) []string {
	entry := regexp.MustCompile(`^([\w.]+)\s*=\s*(\{)?`)
	section := ""
	var tables, all []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "[") {
			name, _, _ := strings.Cut(line[1:], "]")
			section = name + "."
		} else if m := entry.FindStringSubmatch(line); m != nil {
			all = append(all, section+m[1])
			if m[2] != "" {
				tables = append(tables, section+m[1])
			}
		}
	}
	var bad []string
	for _, table := range tables {
		for _, key := range all {
			if strings.HasPrefix(key, table+".") {
				bad = append(bad, table)
				break
			}
		}
	}
	return bad
}

func TestEmbeddedThemesDoNotExtendInlineTables(t *testing.T) {
	if bad := inlineTableExtensions("[menu]\ndropdown = { fg = \"ink\" }\ndropdown.accent = { fg = \"fg\" }\n"); len(bad) != 1 {
		t.Fatalf("old notation not detected: %v", bad)
	}
	files, err := fs.Glob(Files, "*.toml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no embedded themes: %v", err)
	}
	for _, name := range files {
		data, err := Files.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if bad := inlineTableExtensions(string(data)); len(bad) > 0 {
			t.Errorf("%s: write %v as X.base = {...}", name, bad)
		}
	}
}
