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

// Choices returns all built-in Chroma styles sorted by name.
func Choices() []Choice {
	names := styles.Names()
	sort.Strings(names)
	out := make([]Choice, len(names))
	for i, name := range names {
		out[i] = Choice{Name: name, Label: name}
	}
	return out
}

// IndexOf returns the choice index for name, or 0 when not found.
func IndexOf(choices []Choice, name string) int {
	want := strings.TrimSpace(name)
	for i, c := range choices {
		if c.Name == want {
			return i
		}
	}
	return 0
}
