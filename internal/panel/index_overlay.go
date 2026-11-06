package panel

import (
	"context"
	"path/filepath"
	"slices"
	"strings"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// IndexOverlayFunc merges the fileindex into a local listing (fileindex.Index.Overlay): it
// sets each entry's IndexStatus and appends ghost rows for index-only names. panel does not
// import fileindex; the app installs the function. It runs on the listing fetch goroutine.
type IndexOverlayFunc func(ctx context.Context, dir pathloc.Path, entries []fsbackend.Entry) ([]fsbackend.Entry, error)

// IndexDriftRequest asks for the state of the indexed subdirectories of Dir (by name) against
// the index: MaxDepth directory levels are checked, 0 means unlimited (fileindex.Index.DirDrift).
type IndexDriftRequest struct {
	Dir      pathloc.Path
	Subdirs  []string
	MaxDepth int
	// Explicit marks a request made by the user (directory size) rather than the automatic pass.
	Explicit bool
}

// IndexDriftScheduler starts the drift pass off the UI goroutine and reports whether it did.
type IndexDriftScheduler func(req IndexDriftRequest) bool

// driftCandidate reports whether e is a directory a drift pass can say something about.
func driftCandidate(e localfs.Entry) bool {
	return e.Type == localfs.EntryDirectory && slices.Contains([]string{localfs.IndexStatusIndexed, localfs.IndexStatusTagged, localfs.IndexStatusUnchecked}, e.IndexStatus)
}

// requestIndexDrift dispatches one drift pass at maxDepth for the candidate entries that want
// accepts. Only the flat layout shows the result.
func (s *State) requestIndexDrift(maxDepth int, explicit bool, want func(name string) bool) {
	if !s.WithIndex || s.ScheduleIndexDrift == nil || s.ListLayout == ListLayoutTree || !s.Path.IsLocal() {
		return
	}
	var subdirs []string
	for _, e := range s.Entries {
		if driftCandidate(e) && want(e.Name) {
			subdirs = append(subdirs, e.Name)
		}
	}
	if len(subdirs) > 0 {
		s.ScheduleIndexDrift(IndexDriftRequest{Dir: s.Path, Subdirs: subdirs, MaxDepth: maxDepth, Explicit: explicit})
	}
}

// scheduleIndexDrift is the automatic pass after a listing lands: every indexed directory is
// checked down to IndexAutoDriftDepth levels (negative: off).
func (s *State) scheduleIndexDrift() {
	if s.IndexAutoDriftDepth < 0 {
		return
	}
	s.requestIndexDrift(s.IndexAutoDriftDepth, false, func(string) bool { return true })
}

// RequestIndexDrift is the explicit pass of the directory-size requests: the named indexed or
// tagged directories are checked without a depth limit (the walk costs as much as a size scan).
func (s *State) RequestIndexDrift(names ...string) {
	s.requestIndexDrift(0, true, func(name string) bool { return slices.Contains(names, name) })
}

// ApplyIndexDrift applies a drift result (name to differs, unchecked or indexed). differs
// replaces indexed, tagged and unchecked; unchecked replaces indexed only (a tagged directory
// keeps its mark); indexed clears unchecked. Entries of another state (new, missing, excluded)
// are left alone.
func (s *State) ApplyIndexDrift(res map[string]string) {
	changed := false
	for i, e := range s.Entries {
		st, ok := res[e.Name]
		if !ok || !driftCandidate(e) {
			continue
		}
		switch {
		case st == localfs.IndexStatusDiffers,
			st == localfs.IndexStatusUnchecked && e.IndexStatus == localfs.IndexStatusIndexed,
			st == localfs.IndexStatusIndexed && e.IndexStatus == localfs.IndexStatusUnchecked:
			s.Entries[i].IndexStatus = st
			changed = true
		}
	}
	if changed {
		s.rebuildListingByPath()
	}
}

// GhostSources returns the base names of the paths that are ghost rows of a WithIndex panel
// (index-only entries, fileindex:// paths in a local listing). Copy, move, view and edit refuse
// them: there is no file to read. A pure string check, safe on the UI goroutine.
func (s *State) GhostSources(paths ...string) []string {
	if !s.Path.IsLocal() {
		return nil
	}
	var names []string
	for _, p := range paths {
		if strings.HasPrefix(p, pathloc.FileIndexPrefix) {
			names = append(names, filepath.Base(p))
		}
	}
	return names
}

// GhostRefusal is the message for a refused ghost source, or "" when names is empty.
func GhostRefusal(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return "Only in the index: " + strings.Join(names, ", ")
}

// ShowsIndex reports whether the panel shows index data: browsing with fileindex or showing the
// index itself (fileindex:// path).
func (s *State) ShowsIndex() bool {
	return s.WithIndex || s.Path.Scheme() == pathloc.SchemeFileIndex
}

// FollowsIndex reports whether a file operation keeps the fileindex in step with the disk: it was
// started from a panel browsing with fileindex or showing the index (fileindex:// path; p may be
// nil), or it names a fileindex:// path. Operations started from an ordinary local panel leave
// the index alone. Only changes made on the disk reach the index this way (a Browse-with panel's
// local paths, a dedup delete of fileindex:// files on disk); a fileindex:// path handled by the
// index backend changes the index itself. A pure string check, safe on the UI goroutine.
func FollowsIndex(p *State, paths ...pathloc.Path) bool {
	if p != nil && p.ShowsIndex() {
		return true
	}
	for _, loc := range paths {
		if loc.Scheme() == pathloc.SchemeFileIndex {
			return true
		}
	}
	return false
}
