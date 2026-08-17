package panel

import (
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// TreeChildLoadRequest describes an async fetch of a directory's immediate children, dispatched
// by setTreeNodeExpanded the first time a tree-mode row is expanded.
type TreeChildLoadRequest struct {
	DirID string // node ID (absolute path) being expanded
	Loc   pathloc.Path
}

// TreeChildLoadScheduler starts an asynchronous child listing for req. The result is applied via
// ApplyTreeChildLoad on the main event thread. Return false to fall back to loading synchronously
// (nil scheduler convention — e.g. in tests — matching AsyncLoadScheduler).
type TreeChildLoadScheduler func(req TreeChildLoadRequest) bool

// ApplyTreeChildLoad applies the result of an async child fetch dispatched via
// ScheduleTreeChildLoad; call on the main thread. Returns false if the result is stale (the node
// no longer exists — e.g. ApplyListing re-rooted TreeRoots on navigation — or is no longer marked
// Loading, e.g. a superseded duplicate callback) and was silently dropped, or when the apply is
// part of an ExpandAllTreeShallow coalesce (treeExpandQuiet > 0 after decrement) that should not
// rebuild/redraw yet. Returns true when the visible tree was rebuilt and the UI should redraw.
func (s *State) ApplyTreeChildLoad(dirID string, entries []localfs.Entry, err error, viewportRows int) bool {
	node := findTreeNode(s.TreeRoots, dirID)
	if node == nil || !node.Value.Loading {
		return false
	}
	node.Value.Loading = false
	if err != nil {
		node.Value.LoadErr = err
		delete(s.TreeExpanded, dirID)
		return s.finishTreeChildLoadApply(dirID, viewportRows)
	}
	node.Value.LoadErr = nil
	// useDiskPrimary is forced false here (unlike ApplySort's s.primarySortUsesDiskTotals()):
	// disk-usage idle-primary sort stays off for tree children regardless of the panel's
	// current flat-mode sort state — an original Phase 1 design decision, not new scope.
	SortEntries(entries, s.Sort, s.DiskSorter, false)
	node.Children = treeRootsFromEntries(entries)
	if s.TreeExpanded == nil {
		s.TreeExpanded = make(map[string]bool)
	}
	s.TreeExpanded[dirID] = true
	s.scheduleTreeChildGitStatus(dirID, entries)
	// Cascades any remembered-but-not-yet-restored expansion nested under the directory that
	// just finished loading (e.g. a recalled snapshot with several levels deep on an SFTP
	// panel, where each level's children only become available after its own async load).
	s.restoreTreeExpansions()
	return s.finishTreeChildLoadApply(dirID, viewportRows)
}

// finishTreeChildLoadApply coalesces ExpandAllTreeShallow async applies: while treeExpandQuiet
// remains positive, node state is already updated but treeRows/cursor/redraw wait for the last
// in-flight load. Single-row expands (quiet=0) rebuild immediately. Cursor reattaches to
// treeCursorID when set (expand-all anchor, or the row ExpandTreeCursorRow/ToggleTreeExpand
// targeted); otherwise falls back to dirID.
func (s *State) finishTreeChildLoadApply(dirID string, viewportRows int) bool {
	if s.treeExpandQuiet > 0 {
		s.treeExpandQuiet--
		if s.treeExpandQuiet > 0 {
			return false
		}
	}
	if s.treeCursorID == "" {
		s.treeCursorID = dirID
	}
	s.rebuildTreeRows()
	s.reattachTreeCursorByID(s.treeCursorID, viewportRows)
	return true
}

// scheduleTreeChildGitStatus lazily fetches git status for a tree-mode directory's newly-loaded
// children (mirrors prepareGitColumn's guards exactly: skip remote, skip when the git column
// isn't active, skip when no scheduler is wired). GitByPath is a global path-keyed cache, so
// rendering already looks up tree-child rows correctly once populated — this is the only piece
// that was missing. A freshly-loaded tree directory is necessarily under the same work tree as
// the cwd listing that put the panel in an active-git-column state, so this reuses the work-tree
// root prepareGitColumn already resolved (s.gitWorkRoot) instead of recomputing it per directory.
func (s *State) scheduleTreeChildGitStatus(dirID string, entries []localfs.Entry) {
	if s.Path.IsRemote() || !s.GitColumnActive || s.ScheduleGitStatus == nil || len(entries) == 0 {
		return
	}
	paths := make([]gitstatus.ListingPaths, len(entries))
	for i, e := range entries {
		paths[i] = gitstatus.ListingPaths{
			AbsPath: filepath.Clean(e.Path),
			IsDir:   e.Type == localfs.EntryDirectory,
		}
	}
	s.ScheduleGitStatus(GitStatusRequest{
		WorkRoot: s.gitWorkRoot,
		ListDir:  dirID,
		Paths:    paths,
	})
}
