package app

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/textutil"
)

// panelTargetPaths returns sorted selected paths, or the focused entry path.
// The parent (..) row yields nothing.
func panelTargetPaths(p *panel.State) []string {
	if len(p.SelectedPaths) > 0 {
		paths := make([]string, 0, len(p.SelectedPaths))
		for sel := range p.SelectedPaths {
			if s := canonicalTargetPath(sel); s != "" {
				paths = append(paths, s)
			}
		}
		sort.Strings(paths)
		return paths
	}
	entry, ok := p.CurrentEntry()
	if !ok || entry.Name == ".." {
		return nil
	}
	if s := canonicalTargetPath(entry.Path); s != "" {
		return []string{s}
	}
	return nil
}

// panelTargetEntries returns selected entries in sorted path order, or the focused entry.
func panelTargetEntries(p *panel.State) []localfs.Entry {
	if len(p.SelectedPaths) > 0 {
		entries, err := p.SelectedEntries(true, nil)
		if err != nil {
			return nil
		}
		return entries
	}
	entry, ok := p.CurrentEntry()
	if !ok || entry.Name == ".." {
		return nil
	}
	return []localfs.Entry{entry}
}

// hostPathOf maps a panel entry path (host path or fileindex:// URI) to the real local path.
func hostPathOf(raw string) (string, bool) {
	loc, err := pathloc.Parse(raw)
	if err != nil {
		return "", false
	}
	return loc.HostPath()
}

// existingHostDir returns dir, or its nearest existing ancestor directory when dir (an indexed
// path) is gone from disk; "" when none exists.
func existingHostDir(dir string) string {
	for {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func canonicalTargetPath(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	if pl, err := pathloc.Parse(raw); err == nil {
		return pl.String()
	}
	return textutil.AbsPathClean(raw)
}
