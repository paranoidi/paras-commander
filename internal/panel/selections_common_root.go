package panel

import "github.com/paranoidi/paras-commander/internal/pathloc"

// SelectionsCommonRoot folds the parents of the panel's selected paths into their deepest
// common ancestor. multiDir reports whether the selections span more than one parent
// directory. ok is false when there are no selections or they mix schemes/hosts.
func (s *State) SelectionsCommonRoot() (root pathloc.Path, multiDir bool, ok bool) {
	for sel := range s.SelectedPaths {
		loc, err := pathloc.Parse(sel)
		if err != nil {
			return pathloc.Path{}, false, false
		}
		parent := loc.Parent()
		switch {
		case root.IsZero():
			root = parent
		case !parent.Equal(root):
			multiDir = true
			anc, ok := pathloc.CommonAncestor(root, parent)
			if !ok {
				// ponytail: mixed schemes/hosts have no common root; proceed as before
				return pathloc.Path{}, false, false
			}
			root = anc
		}
	}
	return root, multiDir, !root.IsZero()
}
