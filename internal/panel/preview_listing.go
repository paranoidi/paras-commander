package panel

import (
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// ListingSnapshot is a read-only directory listing for carousel side columns.
type ListingSnapshot struct {
	Path    pathloc.Path
	Entries []localfs.Entry
	Cursor  int
	Scroll  int
}

// SnapshotParent returns a parent-directory preview for carousel mode.
// The cursor highlights the directory name of the current path (the exited child).
// The second bool is false when the pane should be blank (filesystem root, load error, pending listing).
func (s *State) SnapshotParent(viewportRows int) (ListingSnapshot, bool) {
	if s.Path.IsZero() || s.ListingPending {
		return ListingSnapshot{}, false
	}
	parent := s.Path.Parent()
	if parent.Equal(s.Path) {
		return ListingSnapshot{}, false
	}
	snap, err := s.buildListingSnapshot(parent, s.Path.Base(), noIndexCursorFallback, viewportRows)
	if err != nil {
		return ListingSnapshot{}, false
	}
	return snap, true
}

// SnapshotChild returns a child-directory preview when the cursor is on a directory.
// The second bool is false when the pane should be blank (file under cursor, load error, pending listing).
func (s *State) SnapshotChild(viewportRows int) (ListingSnapshot, bool) {
	if s.ListingPending {
		return ListingSnapshot{}, false
	}
	entry, ok := s.CurrentEntry()
	if !ok || entry.Type != localfs.EntryDirectory {
		return ListingSnapshot{}, false
	}
	child, err := pathloc.Parse(entry.Path)
	if err != nil {
		return ListingSnapshot{}, false
	}
	selectedName, indexFallback := s.recalledCursorFor(child.String())
	snap, err := s.buildListingSnapshot(child, selectedName, indexFallback, viewportRows)
	if err != nil {
		return ListingSnapshot{}, false
	}
	return snap, true
}

func (s *State) buildListingSnapshot(loc pathloc.Path, selectedName string, indexFallback int, viewportRows int) (ListingSnapshot, error) {
	backendEntries, listingLoc, _, err := s.fetchBackendEntries(loc)
	if err != nil {
		return ListingSnapshot{}, err
	}
	localEntries, err := fsbackend.ToPanelEntries(backendEntries)
	if err != nil {
		return ListingSnapshot{}, err
	}
	entries := append([]localfs.Entry(nil), localEntries...)
	temp := State{
		Entries: entries,
		Sort:    s.Sort,
	}
	temp.ApplySort()
	temp.Cursor = 0
	temp.ScrollOffset = 0
	found := false
	if selectedName != "" {
		found = temp.SelectVisibleEntry(selectedName)
	}
	if !found && indexFallback >= 0 && len(temp.Entries) > 0 {
		if indexFallback >= len(temp.Entries) {
			temp.Cursor = len(temp.Entries) - 1
		} else {
			temp.Cursor = indexFallback
		}
	}
	temp.clampCursor()
	temp.EnsureCursorVisible(viewportRows)
	return ListingSnapshot{
		Path:    listingLoc,
		Entries: temp.Entries,
		Cursor:  temp.Cursor,
		Scroll:  temp.ScrollOffset,
	}, nil
}
