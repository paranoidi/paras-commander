package panel

import (
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

type dirNewFileMarks struct {
	latest   map[string]struct{}
	previous map[string]struct{}
}

// AddNewFileMarks records base names as the latest "new" batch in the listing directory dir.
// Names from the prior latest batch not in this batch move to the previous tier.
func (s *State) AddNewFileMarks(dir pathloc.Path, names []string) {
	if len(names) == 0 {
		return
	}
	key := cleanPathString(dir.String())
	if key == "" {
		return
	}
	if s.NewFileMarksByDir == nil {
		s.NewFileMarksByDir = make(map[string]*dirNewFileMarks)
	}
	dm := s.NewFileMarksByDir[key]
	if dm == nil {
		dm = &dirNewFileMarks{
			latest:   make(map[string]struct{}),
			previous: make(map[string]struct{}),
		}
		s.NewFileMarksByDir[key] = dm
	}
	newLatest := make(map[string]struct{}, len(names))
	for _, n := range names {
		if n == "" {
			continue
		}
		delete(dm.previous, n)
		newLatest[n] = struct{}{}
	}
	for name := range dm.latest {
		if _, inNew := newLatest[name]; !inNew {
			dm.previous[name] = struct{}{}
		}
	}
	dm.latest = newLatest
}

// newlyAppearedNames returns names present in newEntries but absent from oldEntries.
// ponytail: name-only diff, so an external rename (old name gone, new name shows up)
// reads as "new" too — same model AddNewFileMarks already uses for job batches; revisit
// with inode/mtime tracking only if that false positive turns out to matter in practice.
func newlyAppearedNames(oldEntries, newEntries []localfs.Entry) []string {
	oldNames := make(map[string]struct{}, len(oldEntries))
	for _, e := range oldEntries {
		oldNames[e.Name] = struct{}{}
	}
	var added []string
	for _, e := range newEntries {
		if _, ok := oldNames[e.Name]; !ok {
			added = append(added, e.Name)
		}
	}
	return added
}

// clearNewFileMarks drops names from the new-file batches for an already-cleaned dir key
// (see AddRenameMarks: a rename's own reload can misread the new name as newly appeared).
func (s *State) clearNewFileMarks(key string, names []string) {
	dm := s.NewFileMarksByDir[key]
	if dm == nil {
		return
	}
	for _, n := range names {
		delete(dm.latest, n)
		delete(dm.previous, n)
	}
}

// dropNewFileMarks removes session marks for one listing directory.
func (s *State) dropNewFileMarks(dir string) {
	if s.NewFileMarksByDir == nil {
		return
	}
	for k := range s.NewFileMarksByDir {
		if keyUnderDir(k, dir) {
			delete(s.NewFileMarksByDir, k)
		}
	}
}

// keyUnderDir reports whether mark key is dir itself or a directory nested under it (tree layout
// keeps marks for expanded subdirectories).
func keyUnderDir(key, dir string) bool {
	kp, err1 := pathloc.Parse(key)
	dp, err2 := pathloc.Parse(cleanPathString(dir))
	if err1 != nil || err2 != nil {
		return key == cleanPathString(dir)
	}
	return kp.HasPrefix(dp)
}

// markDirKey returns the mark-map key for entry's containing directory: the entry's parent in tree
// layout (rows can live in expanded subdirectories), else the listing directory.
func (s *State) markDirKey(entry localfs.Entry) string {
	if s.ListLayout == ListLayoutTree && entry.Path != "" {
		if p, err := pathloc.Parse(entry.Path); err == nil {
			return cleanPathString(p.Parent().String())
		}
	}
	return cleanPathString(s.Path.String())
}

// NewFileMarkTier reports which new-file suffix tier entry has in the current listing.
func (s *State) NewFileMarkTier(entry localfs.Entry) panellist.NewFileMarkTier {
	if s.NewFileMarksByDir == nil {
		return panellist.NewFileMarkNone
	}
	dm := s.NewFileMarksByDir[s.markDirKey(entry)]
	if dm == nil {
		return panellist.NewFileMarkNone
	}
	if _, ok := dm.latest[entry.Name]; ok {
		return panellist.NewFileMarkLatest
	}
	if _, ok := dm.previous[entry.Name]; ok {
		return panellist.NewFileMarkPrevious
	}
	return panellist.NewFileMarkNone
}
