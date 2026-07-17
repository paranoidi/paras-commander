package panel

import "github.com/paranoidi/paras-commander/internal/pathloc"

// SelectionsCommonRoot folds the parents of the panel's selected paths into their deepest
// common ancestor. multiDir reports whether the selections span more than one parent
// directory. ok is false when there are no selections or they mix schemes/hosts.
func (s *State) SelectionsCommonRoot() (root pathloc.Path, multiDir bool, ok bool) {
	parents := make([]pathloc.Path, 0, len(s.SelectedPaths))
	for sel := range s.SelectedPaths {
		loc, err := pathloc.Parse(sel)
		if err != nil {
			return pathloc.Path{}, false, false
		}
		parents = append(parents, loc.Parent())
	}
	root, multiDir, ok = pathloc.CommonParent(parents)
	return root, multiDir, ok
}
