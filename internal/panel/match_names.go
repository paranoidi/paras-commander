package panel

import (
	"sync/atomic"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// matchNamesGen hands out row generations. It is global so that two State copies carrying the
// same matchNamesRowsGen always hold the same rows.
var matchNamesGen atomic.Uint64

// matchNamesCache is a single-entry cache of NamesDirectlyIn. It is shared by pointer between
// value copies of a State and only touched from the render path on the event loop.
type matchNamesCache struct {
	gen   uint64
	dir   string
	names map[string]struct{}
}

// bumpMatchNamesGen marks the visible rows (Entries, treeRows, filteredIdx) as changed.
func (s *State) bumpMatchNamesGen() {
	s.matchNamesRowsGen = matchNamesGen.Add(1)
}

// NamesDirectlyIn returns the names of the visible rows directly in dir, excluding "..". The map
// is shared: callers must only read it. It is cached per row generation and dir; a State without a
// cache (a literal) or whose rows were set directly (generation 0) rebuilds on every call.
func (s *State) NamesDirectlyIn(dir string) map[string]struct{} {
	cache := s.matchNames
	cacheable := cache != nil && s.matchNamesRowsGen != 0
	if cacheable && cache.gen == s.matchNamesRowsGen && cache.dir == dir {
		return cache.names
	}
	names := make(map[string]struct{}, len(s.Entries))
	add := func(e localfs.Entry) {
		if e.Name != ".." && s.EntryDir(e).String() == dir {
			names[e.Name] = struct{}{}
		}
	}
	if s.ListLayout == ListLayoutTree {
		// Expanded children live only in the tree rows; a filter there hides names from matching.
		for i := range s.VisibleEntryCount() {
			if e, _, ok := s.VisibleEntry(i); ok {
				add(e)
			}
		}
	} else {
		for _, e := range s.Entries {
			add(e)
		}
	}
	if cacheable {
		*cache = matchNamesCache{gen: s.matchNamesRowsGen, dir: dir, names: names}
	}
	return names
}
