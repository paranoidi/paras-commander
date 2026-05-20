package pathloc

import "fmt"

// ParseAll parses each string; returns the first error.
func ParseAll(raw []string) ([]Path, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make([]Path, len(raw))
	for i, s := range raw {
		p, err := Parse(s)
		if err != nil {
			return nil, fmt.Errorf("sources[%d]: %w", i, err)
		}
		out[i] = p
	}
	return out, nil
}

// PathsForTest parses paths for test fixtures (panics on error).
func PathsForTest(raw ...string) []Path {
	out := make([]Path, len(raw))
	for i, s := range raw {
		out[i] = MustParse(s)
	}
	return out
}

// Strings returns canonical strings for each path.
func Strings(paths []Path) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = p.String()
	}
	return out
}
