package panel

import (
	"errors"

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
// The second bool is false when the pane should be blank (filesystem root, load error).
// While a navigation is in flight (ListingPending), this still reads off s.Path/s.Entries — the
// last successfully loaded directory, unchanged until ApplyListing lands — rather than blanking,
// so the parent pane doesn't flash empty for the one frame the async load takes to land.
func (s *State) SnapshotParent(viewportRows int) (ListingSnapshot, bool) {
	if s.Path.IsZero() {
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
// The second bool is false when the pane should be blank (file under cursor, load error).
// While CarouselChildPreviewCoalesce is set, BuildColumns uses the cache only; this method is not
// called. Otherwise this reads off s.Entries/cursor regardless of ListingPending — during a
// pending navigation those still reflect the last successfully loaded directory, so the child
// pane doesn't flash empty for the one frame the async load takes to land.
func (s *State) SnapshotChild(viewportRows int) (ListingSnapshot, bool) {
	if s.CarouselChildPreviewCoalesce {
		if s.CarouselChildCachePaintDuringCoalesce() {
			return s.CarouselSideCache.Child, true
		}
		return ListingSnapshot{}, false
	}
	entry, ok := s.CurrentEntry()
	if !ok || entry.Type != localfs.EntryDirectory {
		s.invalidateCarouselChildCache()
		return ListingSnapshot{}, false
	}
	child, err := pathloc.Parse(entry.Path)
	if err != nil {
		s.invalidateCarouselChildCache()
		return ListingSnapshot{}, false
	}
	selectedName, indexFallback, centerRecalled := s.recalledCursorFor(child.String())
	snap, err := s.buildListingSnapshot(child, selectedName, indexFallback, viewportRows, centerRecalled)
	if err != nil {
		s.invalidateCarouselChildCache()
		return ListingSnapshot{}, false
	}
	target, okTarget := s.carouselChildPreviewTarget()
	if !okTarget {
		s.invalidateCarouselChildCache()
		return ListingSnapshot{}, false
	}
	s.storeCarouselChildCache(snap, true, target)
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

func (s *State) storeCarouselChildCache(snap ListingSnapshot, ok bool, cursorDir string) {
	if ok {
		s.CarouselSideCache.Child = snap
		s.CarouselSideCache.ChildOK = true
		s.CarouselSideCache.ChildCursorDir = cursorDir
		return
	}
	s.invalidateCarouselChildCache()
}

// SnapshotDirectory builds a sorted listing for dir using s for list/sort/backend settings and
// recallSources for HistoryCursorByPath (same highlight rules as Enter / carousel child pane).
func (s *State) SnapshotDirectory(dir string, viewportRows int, recallSources ...*State) (ListingSnapshot, error) {
	canonical := cleanPathString(dir)
	if canonical == "" {
		return ListingSnapshot{}, errors.New("empty directory path")
	}
	loc, err := pathloc.Parse(canonical)
	if err != nil {
		return ListingSnapshot{}, err
	}
	name, idx, recalled := BestRecalledCursor(canonical, recallSources...)
	if name == "" && idx == noIndexCursorFallback {
		if n, ok := selectionBasenameUnderDirectory(s, canonical); ok {
			name = n
			recalled = true
		} else {
			for _, st := range recallSources {
				if st == nil {
					continue
				}
				if n, ok := selectionBasenameUnderDirectory(st, canonical); ok {
					name = n
					recalled = true
					break
				}
			}
		}
	}
	return s.buildListingSnapshot(loc, name, idx, viewportRows, recalled)
}

func selectionBasenameUnderDirectory(st *State, dir string) (string, bool) {
	if st == nil || len(st.SelectedPaths) == 0 {
		return "", false
	}
	dir = cleanPathString(dir)
	if dir == "" {
		return "", false
	}
	for p := range st.SelectedPaths {
		cp := cleanPathString(p)
		if cp == "" || cp == dir {
			continue
		}
		if !isStrictPathDescendant(dir, cp) {
			continue
		}
		loc, err := pathloc.Parse(cp)
		if err != nil {
			continue
		}
		return loc.Base(), true
	}
	return "", false
}

// ListingAtPath reports whether st is a loaded listing for canonical dir.
func (s *State) ListingAtPath(dir string) bool {
	if s == nil || s.ListingPending {
		return false
	}
	return cleanPathString(s.Path.String()) == cleanPathString(dir) && s.VisibleEntryCount() > 0
}

// CloneListingFrom copies listing rows, cursor, scroll, and volume/git snapshot fields from src.
func (dst *State) CloneListingFrom(src *State) {
	if dst == nil || src == nil {
		return
	}
	dst.Path = src.Path
	dst.Entries = append([]localfs.Entry(nil), src.Entries...)
	dst.Cursor = src.Cursor
	dst.ScrollOffset = src.ScrollOffset
	dst.VolumeSpaceOK = src.VolumeSpaceOK
	dst.VolumeAvailBytes = src.VolumeAvailBytes
	dst.VolumeTotalBytes = src.VolumeTotalBytes
	dst.ListingDevice = src.ListingDevice
	dst.ListingDeviceValid = src.ListingDeviceValid
	dst.GitignoreActive = src.GitignoreActive
	dst.DotfilesHiddenActive = src.DotfilesHiddenActive
	dst.GitColumnActive = src.GitColumnActive
	dst.GitPending = src.GitPending
	dst.GitByPath = src.GitByPath
	dst.ListingPending = false
}

func (s *State) buildListingSnapshot(loc pathloc.Path, selectedName string, indexFallback int, viewportRows int, centerRecalled bool) (ListingSnapshot, error) {
	backendEntries, listingLoc, _, _, err := s.fetchBackendEntries(loc)
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
	centerOnHighlight := centerRecalled || selectedName != "" || indexFallback >= 0
	temp.applyHighlightScroll(viewportRows, centerOnHighlight)
	return ListingSnapshot{
		Path:    listingLoc,
		Entries: temp.Entries,
		Cursor:  temp.Cursor,
		Scroll:  temp.ScrollOffset,
	}, nil
}
