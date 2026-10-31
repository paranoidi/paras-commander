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

// IndexDriftRequest asks for the indexed subdirectories of Dir (by name) that differ from the
// index somewhere below.
type IndexDriftRequest struct {
	Dir     pathloc.Path
	Subdirs []string
}

// IndexDriftScheduler starts the drift pass off the UI goroutine and reports whether it did.
type IndexDriftScheduler func(req IndexDriftRequest) bool

// scheduleIndexDrift dispatches the directory-drift pass for the current listing of a
// WithIndex panel. Only the flat layout shows the result.
func (s *State) scheduleIndexDrift() {
	if !s.WithIndex || s.ScheduleIndexDrift == nil || s.ListLayout == ListLayoutTree || !s.Path.IsLocal() {
		return
	}
	var subdirs []string
	for _, e := range s.Entries {
		if e.Type == localfs.EntryDirectory && (e.IndexStatus == localfs.IndexStatusIndexed || e.IndexStatus == localfs.IndexStatusTagged) {
			subdirs = append(subdirs, e.Name)
		}
	}
	if len(subdirs) > 0 {
		s.ScheduleIndexDrift(IndexDriftRequest{Dir: s.Path, Subdirs: subdirs})
	}
}

// ApplyIndexDrift marks the named indexed directories as differing from the index. Entries of
// another state (new, missing, excluded) are left alone.
func (s *State) ApplyIndexDrift(dirs map[string]bool) {
	changed := false
	for i, e := range s.Entries {
		if dirs[e.Name] && e.Type == localfs.EntryDirectory && slices.Contains([]string{localfs.IndexStatusIndexed, localfs.IndexStatusTagged}, e.IndexStatus) {
			s.Entries[i].IndexStatus = localfs.IndexStatusDiffers
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
