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

// SnapshotParent returns the cached parent-directory preview for carousel mode. It never touches
// the filesystem: the cache is populated asynchronously (see internal/app's carousel snapshot
// dispatch, triggered on center-directory change) and this is a pure read, so painting the
// carousel never blocks the UI on I/O. The second bool is false only when there is structurally no
// parent to show (no path yet, or already at the filesystem root). Otherwise this keeps returning
// the last cached preview even while it's stale for the current center directory (an async fetch
// for the new target is in flight) — showing briefly-stale content instead of blanking avoids a
// flash on every folder change, the same hold-stale-content-while-loading pattern used elsewhere
// (quick view, the file preview panel); CarouselParentCacheValid is for the dispatch side to decide
// whether a fresh fetch is needed, not for gating what gets painted here.
func (s *State) SnapshotParent() (ListingSnapshot, bool) {
	if s.Path.IsZero() {
		return ListingSnapshot{}, false
	}
	if s.Path.Parent().Equal(s.Path) {
		return ListingSnapshot{}, false
	}
	return s.CarouselSideCache.Parent, s.CarouselSideCache.ParentOK
}

// SnapshotChild returns the cached child-directory preview when the cursor is on a directory. It
// is a pure cache read on the same terms as SnapshotParent (see there); the second bool is false
// only when the cursor is structurally not on a directory.
func (s *State) SnapshotChild() (ListingSnapshot, bool) {
	if s.CarouselChildPreviewCoalesce {
		if s.CarouselChildCachePaintDuringCoalesce() {
			return s.CarouselSideCache.Child, true
		}
		return ListingSnapshot{}, false
	}
	entry, ok := s.CurrentEntry()
	if !ok || entry.Type != localfs.EntryDirectory {
		return ListingSnapshot{}, false
	}
	return s.CarouselSideCache.Child, s.CarouselSideCache.ChildOK
}

// PreviewCursorRecall computes the cursor to highlight when opening dir in a preview/overlay
// listing, mirroring the recall rules used for Enter / carousel child pane: a per-panel
// remembered cursor from sources' navigation history first (BestRecalledCursor), then a
// fallback to a selected path under dir, checked against sources in order. Pure and IO-free.
func PreviewCursorRecall(dir string, sources ...*State) (name string, idx int) {
	canonical := cleanPathString(dir)
	name, idx, _ = BestRecalledCursor(canonical, sources...)
	if name != "" || idx != noIndexCursorFallback {
		return name, idx
	}
	for _, st := range sources {
		if st == nil {
			continue
		}
		if n, ok := selectionBasenameUnderDirectory(st, canonical); ok {
			return n, idx
		}
	}
	return name, idx
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
	return s.BuildListingSnapshotFromEntries(listingLoc, backendEntries, selectedName, indexFallback, viewportRows, centerRecalled)
}

// BuildListingSnapshotFromEntries builds a sorted, cursor-centered listing snapshot from entries
// already fetched off-thread. It is the tail half of buildListingSnapshot, split out so an async
// carousel side-column fetch can do the filesystem read on a background goroutine and defer this
// step — which reads s.Sort/IdleDiskTotalsSort/DiskSorter — to the owning (UI) goroutine once the
// entries are back.
func (s *State) BuildListingSnapshotFromEntries(loc pathloc.Path, rawEntries []fsbackend.Entry, selectedName string, indexFallback int, viewportRows int, centerRecalled bool) (ListingSnapshot, error) {
	localEntries, err := fsbackend.ToPanelEntries(rawEntries)
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
		Path:    loc,
		Entries: temp.Entries,
		Cursor:  temp.Cursor,
		Scroll:  temp.ScrollOffset,
	}, nil
}
