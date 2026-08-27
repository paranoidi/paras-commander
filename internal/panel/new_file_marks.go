package panel

import (
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

type dirNewFileMarks struct {
	latest   map[string]struct{}
	previous map[string]struct{}
	// pendingRemoval holds names optimistically pruned from the listing by an in-flight
	// move/delete/flatten job (see RemoveEntriesByPath). A stale reload that still sees
	// one of these names on disk before the job's real op lands must not read it as
	// newly created; filterPendingRemoval suppresses that and self-cleans once the name
	// is confirmed actually gone.
	pendingRemoval map[string]struct{}
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

// MarkPendingRemoval records names as optimistically removed from the listing directory dir by
// an in-flight move/delete/flatten job, so a stale reload racing that job's completion doesn't
// misread their reappearance as newly created.
func (s *State) MarkPendingRemoval(dir pathloc.Path, names []string) {
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
	if dm.pendingRemoval == nil {
		dm.pendingRemoval = make(map[string]struct{}, len(names))
	}
	for _, n := range names {
		if n == "" {
			continue
		}
		dm.pendingRemoval[n] = struct{}{}
	}
}

// filterPendingRemoval drops names pending removal in dir from newlyAppeared (suppressing the
// false "new" mark), and clears any pending-removal name absent from freshEntries (the disk
// listing just confirmed it's actually gone, so there is nothing left to suppress).
func (s *State) filterPendingRemoval(dir pathloc.Path, newlyAppeared []string, freshEntries []localfs.Entry) []string {
	key := cleanPathString(dir.String())
	dm := s.NewFileMarksByDir[key]
	if dm == nil || len(dm.pendingRemoval) == 0 {
		return newlyAppeared
	}
	present := make(map[string]struct{}, len(freshEntries))
	for _, e := range freshEntries {
		present[e.Name] = struct{}{}
	}
	for n := range dm.pendingRemoval {
		if _, ok := present[n]; !ok {
			delete(dm.pendingRemoval, n)
		}
	}
	if len(newlyAppeared) == 0 {
		return newlyAppeared
	}
	filtered := newlyAppeared[:0:0]
	for _, n := range newlyAppeared {
		if _, ok := dm.pendingRemoval[n]; ok {
			continue
		}
		filtered = append(filtered, n)
	}
	return filtered
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
