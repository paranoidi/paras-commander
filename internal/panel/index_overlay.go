package panel

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"time"

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
	// Seq identifies the pass: the scheduler hands it to FinishIndexDrift however the pass ends.
	Seq uint64
}

// IndexDriftScheduler starts the drift pass off the UI goroutine and reports whether it did. A
// started pass must end in FinishIndexDrift, whether it succeeds, fails or is cancelled.
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
	var subdirs, paths []string
	for _, e := range s.Entries {
		if driftCandidate(e) && want(e.Name) {
			subdirs = append(subdirs, e.Name)
			paths = append(paths, e.Path)
		}
	}
	if len(subdirs) == 0 {
		return
	}
	s.indexDriftSeq++
	if !s.ScheduleIndexDrift(IndexDriftRequest{Dir: s.Path, Subdirs: subdirs, MaxDepth: maxDepth, Explicit: explicit, Seq: s.indexDriftSeq}) {
		return
	}
	if s.indexDriftPending == nil {
		s.indexDriftPending = make(map[string]uint64, len(paths))
	}
	for _, p := range paths {
		s.indexDriftPending[p] = s.indexDriftSeq
	}
}

// FinishIndexDrift ends the wait on the whole drift pass seq, leaving directories a newer pass
// also covers, and reports whether any row stopped waiting.
func (s *State) FinishIndexDrift(seq uint64) bool {
	changed := false
	for p, pending := range s.indexDriftPending {
		if pending == seq {
			delete(s.indexDriftPending, p)
			changed = true
		}
	}
	return changed
}

// FinishIndexDriftDirs ends the wait on the named rows of the listing, each only if the given
// pass (name to Seq) is the newest that covers it.
func (s *State) FinishIndexDriftDirs(seqs map[string]uint64) {
	for _, e := range s.Entries {
		if seq, ok := seqs[e.Name]; ok && s.indexDriftPending[e.Path] == seq {
			delete(s.indexDriftPending, e.Path)
		}
	}
}

// IndexDriftPending reports whether the row is a directory whose state a running drift pass has
// yet to settle. Rows outside the drift candidates, differs among them, already have their answer.
func (s *State) IndexDriftPending(e localfs.Entry) bool {
	_, pending := s.indexDriftPending[e.Path]
	return pending && driftCandidate(e)
}

// scheduleIndexDrift is the automatic pass after a listing lands: every indexed directory not in
// carried is checked down to IndexAutoDriftDepth levels (negative: off).
func (s *State) scheduleIndexDrift(carried map[string]bool) {
	if s.IndexAutoDriftDepth < 0 {
		return
	}
	s.requestIndexDrift(s.IndexAutoDriftDepth, false, func(name string) bool { return !carried[name] })
}

// indexListedDir is a drift-candidate directory as the overlay listed it.
type indexListedDir struct {
	status string
	mod    time.Time
}

// carryIndexDrift keeps the drift results of a same-directory reload: a directory listed again
// with the same mtime and overlay state takes the state the previous listing's row ended with,
// and its name is returned so the automatic pass skips it. A change beside it (a file deleted in
// the directory) says nothing about its tree, so re-walking every directory would only flash
// every row pending. It records entries for the next reload; entries is the incoming listing.
func (s *State) carryIndexDrift(sameDir bool, entries []localfs.Entry) map[string]bool {
	prior := s.indexListed
	s.indexListed = nil
	var shown map[string]string
	if sameDir && len(prior) > 0 {
		shown = make(map[string]string, len(prior))
		for _, e := range s.Entries {
			if _, ok := prior[e.Name]; ok && e.Type == localfs.EntryDirectory {
				shown[e.Name] = e.IndexStatus
			}
		}
	}
	var carried map[string]bool
	for i, e := range entries {
		if !driftCandidate(e) {
			continue
		}
		listed := indexListedDir{status: e.IndexStatus, mod: e.ModifiedAt}
		if s.indexListed == nil {
			s.indexListed = map[string]indexListedDir{}
		}
		s.indexListed[e.Name] = listed
		st, ok := shown[e.Name]
		if !ok || prior[e.Name].status != listed.status || !prior[e.Name].mod.Equal(listed.mod) {
			continue
		}
		entries[i].IndexStatus = st
		if carried == nil {
			carried = map[string]bool{}
		}
		carried[e.Name] = true
	}
	return carried
}

// ForgetIndexDrift makes the next listing walk every indexed directory again: the index changed
// in a way a directory's own row does not show (a scan ended, a prune).
func (s *State) ForgetIndexDrift() { s.indexListed = nil }

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
