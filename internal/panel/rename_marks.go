package panel

import (
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// AddRenameMarks records base names as recently renamed in the listing directory dir.
func (s *State) AddRenameMarks(dir pathloc.Path, names []string) {
	if len(names) == 0 {
		return
	}
	key := cleanPathString(dir.String())
	if key == "" {
		return
	}
	if s.RenameMarksByDir == nil {
		s.RenameMarksByDir = make(map[string]map[string]struct{})
	}
	marks := s.RenameMarksByDir[key]
	if marks == nil {
		marks = make(map[string]struct{})
		s.RenameMarksByDir[key] = marks
	}
	for _, n := range names {
		if n == "" {
			continue
		}
		marks[n] = struct{}{}
	}
}

// dropRenameMarks removes session rename marks for one listing directory.
func (s *State) dropRenameMarks(dir string) {
	if s.RenameMarksByDir == nil {
		return
	}
	delete(s.RenameMarksByDir, cleanPathString(dir))
}

// IsRenameMarked reports whether entry was recently renamed in the current listing.
func (s *State) IsRenameMarked(entry localfs.Entry) bool {
	if s.RenameMarksByDir == nil {
		return false
	}
	marks := s.RenameMarksByDir[cleanPathString(s.Path.String())]
	if marks == nil {
		return false
	}
	_, ok := marks[entry.Name]
	return ok
}
