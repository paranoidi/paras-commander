// Package pathpick validates and resolves path-picker query strings.
package pathpick

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// QueryLooksPathlike reports whether q should be validated with Lstat.
func QueryLooksPathlike(q string) bool {
	q = strings.TrimSpace(q)
	if q == "" {
		return false
	}
	if strings.HasPrefix(q, "~") {
		return true
	}
	if filepath.IsAbs(q) {
		return true
	}
	for _, r := range q {
		if r == '/' || r == filepath.Separator {
			return true
		}
	}
	return strings.HasPrefix(q, ".")
}

// ExpandTilde expands a leading ~ in q using home.
func ExpandTilde(home, q string) string {
	q = strings.TrimSpace(q)
	switch {
	case q == "~":
		if home == "" {
			return q
		}
		return home
	case strings.HasPrefix(q, "~/"):
		if home == "" {
			return filepath.Clean(q)
		}
		return filepath.Join(home, q[len("~/"):])
	default:
		return q
	}
}

// ResolveQuery resolves raw against panelPath and home (tilde, relative, absolute).
func ResolveQuery(panelPath, home, raw string) string {
	q := ExpandTilde(home, raw)
	q = strings.TrimSpace(q)
	if filepath.IsAbs(q) {
		return filepath.Clean(q)
	}
	return filepath.Clean(filepath.Join(panelPath, q))
}

// ResolveDestination resolves a typed Copy/Move/Flatten/Extract destination exactly as the dialogs
// validate it (ResolveQuery): sftp:// and fileindex:// URIs as given, a leading ~ against home, and a relative
// path against the panel directory.
func ResolveDestination(panel pathloc.Path, home, raw string) (pathloc.Path, error) {
	raw = strings.TrimSpace(raw)
	if pathloc.HasURIPrefix(raw) || panel.IsRemote() {
		// ponytail: relative/~ destinations typed on a remote panel are not resolved against the
		// remote directory; join via pathloc when needed.
		return pathloc.Parse(raw)
	}
	if raw == "" {
		return pathloc.Parse(raw)
	}
	return pathloc.Parse(ResolveQuery(panel.String(), home, raw))
}

// TypedDoesNotExist returns true when raw looks like a path and Stat reports missing.
func TypedDoesNotExist(panelPath, home, raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if pathloc.HasURIPrefix(raw) {
		loc, err := pathloc.Parse(raw)
		if err != nil {
			return true
		}
		_, err = fsbackend.Default().Stat(context.Background(), loc)
		return err != nil
	}
	if !QueryLooksPathlike(raw) {
		return false
	}
	resolved := ResolveQuery(panelPath, home, raw)
	_, err := os.Lstat(resolved)
	return err != nil
}
