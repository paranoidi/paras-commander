package themes

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestDefaultThemeUsesOnly16ANSIColors keeps default.toml on the terminal's 16 ANSI colors: every
// [palette] value is an index 0–15 (or "default") and no style names a hex color directly.
func TestDefaultThemeUsesOnly16ANSIColors(t *testing.T) {
	data, err := Files.ReadFile("default.toml")
	if err != nil {
		t.Fatal(err)
	}
	section := ""
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
		if section != "[palette]" {
			continue
		}
		m := entry.FindStringSubmatch(trimmed)
		if m == nil {
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
