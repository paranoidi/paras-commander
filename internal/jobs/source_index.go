package jobs

import (
	"github.com/paranoidi/paras-commander/internal/oplog"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// sourceIndex maps an item path to the index of the job source it lies under, so each
// operation record names its source (the jobs view's per-source sub-rows and Operations
// filter). It is the one place a path becomes a source index. Lookup walks the path's
// ancestors, O(path depth), because it runs once per file.
type sourceIndex struct {
	bySource map[string]int32
}

// newSourceIndex indexes sources; nil for fewer than two (records then carry no source).
func newSourceIndex(sources []pathloc.Path) *sourceIndex {
	if len(sources) < 2 {
		return nil
	}
	m := make(map[string]int32, len(sources))
	for i, s := range sources {
		if _, dup := m[s.String()]; !dup {
			m[s.String()] = int32(i)
		}
	}
	return &sourceIndex{bySource: m}
}

// indexOf returns the index of the source p equals or lies under, else oplog.NoSource.
func (x *sourceIndex) indexOf(p pathloc.Path) int32 {
	if x == nil || p.IsZero() {
		return oplog.NoSource
	}
	for {
		if i, ok := x.bySource[p.String()]; ok {
			return i
		}
		parent := p.Parent()
		if parent.IsZero() || parent.Equal(p) {
			return oplog.NoSource
		}
		p = parent
	}
}
