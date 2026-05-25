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
		s.storeCarouselParentCache(ListingSnapshot{}, false)
		return ListingSnapshot{}, false
	}
	parent := s.Path.Parent()
	if parent.Equal(s.Path) {
		s.storeCarouselParentCache(ListingSnapshot{}, false)
		return ListingSnapshot{}, false
	}
	snap, err := s.buildListingSnapshot(parent, s.Path.Base(), noIndexCursorFallback, viewportRows, false)
	if err != nil {
		s.storeCarouselParentCache(ListingSnapshot{}, false)
		return ListingSnapshot{}, false
	}
	s.storeCarouselParentCache(snap, true)
	return snap, true
}

// SnapshotChild returns a child-directory preview when the cursor is on a directory.
// The second bool is false when the pane should be blank (file under cursor, load error, pending listing).
// While CarouselChildPreviewCoalesce is set, BuildColumns uses the cache only; this method is not called.
func (s *State) SnapshotChild(viewportRows int) (ListingSnapshot, bool) {
	if s.CarouselChildPreviewCoalesce {
		if s.CarouselSideCache.ChildOK {
			return s.CarouselSideCache.Child, true
		}
		return ListingSnapshot{}, false
	}
	if s.ListingPending {
		s.storeCarouselChildCache(ListingSnapshot{}, false)
		return ListingSnapshot{}, false
	}
	entry, ok := s.CurrentEntry()
	if !ok || entry.Type != localfs.EntryDirectory {
		s.storeCarouselChildCache(ListingSnapshot{}, false)
		return ListingSnapshot{}, false
	}
	child, err := pathloc.Parse(entry.Path)
	if err != nil {
		s.storeCarouselChildCache(ListingSnapshot{}, false)
		return ListingSnapshot{}, false
	}
	selectedName, indexFallback, centerRecalled := s.recalledCursorFor(child.String())
	snap, err := s.buildListingSnapshot(child, selectedName, indexFallback, viewportRows, centerRecalled)
	if err != nil {
		s.storeCarouselChildCache(ListingSnapshot{}, false)
		return ListingSnapshot{}, false
	}
	s.storeCarouselChildCache(snap, true)
	return snap, true
}

func (s *State) storeCarouselParentCache(snap ListingSnapshot, ok bool) {
	if ok {
		s.CarouselSideCache.Parent = snap
		s.CarouselSideCache.ParentOK = true
		return
	}
	s.CarouselSideCache.ParentOK = false
}

func (s *State) storeCarouselChildCache(snap ListingSnapshot, ok bool) {
	if ok {
		s.CarouselSideCache.Child = snap
		s.CarouselSideCache.ChildOK = true
		return
	}
	s.CarouselSideCache.ChildOK = false
}

func (s *State) buildListingSnapshot(loc pathloc.Path, selectedName string, indexFallback int, viewportRows int, centerRecalled bool) (ListingSnapshot, error) {
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
		Entries:            entries,
		Sort:               s.Sort,
		IdleDiskTotalsSort: s.IdleDiskTotalsSort,
		DiskSorter:         s.DiskSorter,
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
	if centerRecalled || selectedName != "" || indexFallback >= 0 {
		temp.EnsureCursorCentered(viewportRows)
	} else {
		temp.EnsureCursorVisible(viewportRows)
	}
	return ListingSnapshot{
		Path:    listingLoc,
		Entries: temp.Entries,
		Cursor:  temp.Cursor,
		Scroll:  temp.ScrollOffset,
	}, nil
}
