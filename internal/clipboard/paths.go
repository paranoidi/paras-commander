package clipboard

import (
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// BuildFileURLs joins canonical path strings (one per path) with newlines.
func BuildFileURLs(paths []string) string {
	return joinLines(mapPaths(paths, pathURL))
}

// BuildDirURLs joins parent directory URLs; when no dirname can be derived, panelDir is used.
func BuildDirURLs(paths []string, panelDir string) string {
	fallback := pathURL(panelDir)
	lines := make([]string, 0, len(paths))
	for _, raw := range paths {
		line := dirURL(raw)
		if line == "" {
			line = fallback
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 && fallback != "" {
		return fallback
	}
	return strings.Join(lines, "\n")
}

// BuildFilenames joins entry basenames with newlines (skips ".." rows).
func BuildFilenames(entries []localfs.Entry) string {
	return joinLines(mapEntryNames(entries, false))
}

// BuildFilenamesWithoutExt joins entry stems with newlines (skips ".." rows).
func BuildFilenamesWithoutExt(entries []localfs.Entry) string {
	return joinLines(mapEntryNames(entries, true))
}

func pathURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	p, err := pathloc.Parse(raw)
	if err != nil {
		return filepath.Clean(raw)
	}
	return p.String()
}

func dirURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	p, err := pathloc.Parse(raw)
	if err != nil {
		parent := filepath.Dir(filepath.Clean(raw))
		if parent == "" || parent == "." {
			return ""
		}
		return parent
	}
	parent := p.Parent()
	if parent.IsZero() {
		return ""
	}
	return parent.String()
}

func mapPaths(paths []string, fn func(string) string) []string {
	out := make([]string, 0, len(paths))
	for _, raw := range paths {
		if s := fn(raw); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func mapEntryNames(entries []localfs.Entry, withoutExt bool) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Name == ".." {
			continue
		}
		name := e.Name
		if withoutExt {
			name = strings.TrimSuffix(name, filepath.Ext(name))
		}
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}
