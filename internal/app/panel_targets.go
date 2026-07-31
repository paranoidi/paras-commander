package app

import (
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

func canonicalTargetPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if pl, err := pathloc.Parse(raw); err == nil {
		return pl.String()
	}
	return textutil.AbsPathClean(raw)
}
