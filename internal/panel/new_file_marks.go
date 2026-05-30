package panel

import (
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// AddNewFileMarks records base names as "new" in the listing directory dir.
func (s *State) AddNewFileMarks(dir pathloc.Path, names []string) {
	if len(names) == 0 {
		return
	}
	key := cleanPathString(dir.String())
	if key == "" {
		return
	}
	if s.NewFileMarksByDir == nil {
		s.NewFileMarksByDir = make(map[string]map[string]struct{})
	}
	set := s.NewFileMarksByDir[key]
	if set == nil {
		set = make(map[string]struct{})
		s.NewFileMarksByDir[key] = set
	}
	for _, n := range names {
		if n == "" {
			continue
		}
		set[n] = struct{}{}
	}
}

// dropNewFileMarks removes session marks for one listing directory.
func (s *State) dropNewFileMarks(dir string) {
	if s.NewFileMarksByDir == nil {
		return
	}
	delete(s.NewFileMarksByDir, cleanPathString(dir))
}

// HasNewFileMark reports whether entry should show the new-file suffix in the current listing.
func (s *State) HasNewFileMark(entry localfs.Entry) bool {
	if s.NewFileMarksByDir == nil {
		return false
	}
	set := s.NewFileMarksByDir[cleanPathString(s.Path.String())]
	if len(set) == 0 {
		return false
	}
	_, ok := set[entry.Name]
	return ok
}
