package chromastyles

import (
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2/styles"
)

// Choice is one selectable Chroma highlighting style.
type Choice struct {
	Name  string
	Label string
}

// Choices returns built-in and custom Chroma styles sorted by name.
func Choices() []Choice {
	names := make(map[string]struct{}, len(styles.Registry)+len(custom))
	for name := range styles.Registry {
		names[name] = struct{}{}
	}
	for name := range custom {
		names[name] = struct{}{}
	}

	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)

	out := make([]Choice, len(sorted))
	for i, name := range sorted {
		label := name
		if style, ok := custom[name]; ok && strings.TrimSpace(style.Name) != "" {
			label = style.Name
		}
		out[i] = Choice{Name: name, Label: label}
	}
	return out
}

// IndexOf returns the choice index for name, or 0 when not found.
func IndexOf(choices []Choice, name string) int {
	want := strings.ToLower(strings.TrimSpace(name))
	for i, c := range choices {
		if c.Name == want {
			return i
		}
	}
	return 0
}
