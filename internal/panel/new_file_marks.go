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

// dropNewFileMarks removes session marks for one listing directory.
func (s *State) dropNewFileMarks(dir string) {
	if s.NewFileMarksByDir == nil {
		return
	}
	delete(s.NewFileMarksByDir, cleanPathString(dir))
}

// NewFileMarkTier reports which new-file suffix tier entry has in the current listing.
func (s *State) NewFileMarkTier(entry localfs.Entry) panellist.NewFileMarkTier {
	if s.NewFileMarksByDir == nil {
		return panellist.NewFileMarkNone
	}
	dm := s.NewFileMarksByDir[cleanPathString(s.Path.String())]
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
