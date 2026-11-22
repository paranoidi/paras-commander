package app

import (
	"slices"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// followTarget is where a view's embedded panel follows a row of the view: the directory to list
// and the entry to put the cursor on ("" for the directory itself). The zero value follows nothing.
type followTarget struct{ dir, name string }

// followTargetOf is the target of a row at path: the directory itself for a dir row, else the file's
// parent (scheme kept) with the cursor on the file.
func followTargetOf(path string, isDir, ok bool) followTarget {
	switch {
	case !ok || path == "":
		return followTarget{}
	case isDir:
		return followTarget{dir: path}
	}
	dir, name := pathloc.SplitDirBase(path)
	return followTarget{dir: dir, name: name}
}

// panelFollow remembers the targets an embedded panel last followed, one per row source of the
// view, so only a cursor move re-syncs the panel (switching panes does not) and manual navigation
// inside it survives until then.
type panelFollow struct{ prev []followTarget }

// next returns the target of the first source, trying prefer first, that changed since the last call
// (ok false: none did). Every source's current target is remembered.
func (f *panelFollow) next(prefer int, targets ...followTarget) (followTarget, bool) {
	prev := f.prev
	f.prev = slices.Clone(targets)
	for n := range targets {
		i := (prefer + n) % len(targets)
		if targets[i] != (followTarget{}) && (i >= len(prev) || targets[i] != prev[i]) {
			return targets[i], true
		}
	}
	return followTarget{}, false
}

// reset forgets what was followed, so the next call follows afresh.
func (f *panelFollow) reset() { f.prev = nil }

// followLoad lists t in the embedded panel p (registry ID id).
func (a *App) followLoad(p *panel.State, id int, t followTarget) error {
	return p.LoadWithViewport(t.dir, t.name, a.panelViewportRows(id), 0)
}
