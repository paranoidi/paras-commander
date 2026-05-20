package ops

import "github.com/paranoidi/paras-commander/internal/pathloc"

// MustPaths parses paths for tests; panics on error.
func MustPaths(raw ...string) []pathloc.Path {
	out, err := pathloc.ParseAll(raw)
	if err != nil {
		panic(err)
	}
	return out
}

// MustPath parses one path for tests; panics on error.
func MustPath(raw string) pathloc.Path {
	return pathloc.MustParse(raw)
}
