package panelcarousel

import (
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
)

// ColumnKind identifies a carousel pane.
type ColumnKind int

const (
	ColumnParent ColumnKind = iota
	ColumnCenter
	ColumnChild
)

// ChildPreviewKind selects what the carousel child (rightmost) column shows.
type ChildPreviewKind int

const (
	ChildPreviewNone ChildPreviewKind = iota
	ChildPreviewDirectoryListing
	ChildPreviewFile
)

// Column is one carousel pane (parent preview, live center, or child preview).
type Column struct {
	Kind      ColumnKind
	Populated bool
	Active    bool
	Snapshot  panel.ListingSnapshot
}

func centerCursorOnFile(center panel.State) bool {
	entry, ok := center.CurrentEntry()
	return ok && entry.Type != localfs.EntryDirectory
}

// ChildPreviewKindFor reports what the child column should show for the current center highlight.
func ChildPreviewKindFor(center panel.State, quickViewEnabled bool, filePreviewEligible bool) ChildPreviewKind {
	if quickViewEnabled {
		return ChildPreviewNone
	}
	entry, ok := center.CurrentEntry()
	if ok && entry.Type == localfs.EntryDirectory {
		if center.CarouselCenterHasSubdirectories() {
			return ChildPreviewDirectoryListing
		}
		return ChildPreviewNone
	}
	if centerCursorOnFile(center) {
		if filePreviewEligible {
			return ChildPreviewFile
		}
		if center.CarouselCenterHasSubdirectories() {
			return ChildPreviewNone
		}
		return ChildPreviewNone
	}
	if center.CarouselCenterHasSubdirectories() {
		return ChildPreviewDirectoryListing
	}
	return ChildPreviewNone
}

// ShowChildPreviewColumn reports whether the carousel child (preview) column should be built and painted.
// It is omitted when Quick view already previews the inactive panel. Otherwise it appears when the center
// listing has subdirectories or when file preview is eligible and the cursor is on a file.
func ShowChildPreviewColumn(center panel.State, quickViewEnabled bool, filePreviewEligible bool) bool {
	if quickViewEnabled {
		return false
	}
	if center.CarouselCenterHasSubdirectories() {
		return true
	}
	return filePreviewEligible && centerCursorOnFile(center)
}

// BuildColumns constructs parent/center/child column descriptors from the live panel state.
// While CarouselChildPreviewCoalesce is set, the child column reuses the last cached snapshot
// and does not read the filesystem; callers paint the child column only after coalesce ends.
// File preview uses separate Model.CarouselFilePreview state; child is not populated for that kind.
func BuildColumns(center panel.State, viewportRows int, quickViewEnabled bool, filePreviewEligible bool) (parent, mid, child Column, childKind ChildPreviewKind) {
	mid = Column{Kind: ColumnCenter, Populated: true, Active: true}
	childKind = ChildPreviewKindFor(center, quickViewEnabled, filePreviewEligible)
	if snap, ok := center.SnapshotParent(viewportRows); ok {
		parent = Column{Kind: ColumnParent, Populated: true, Snapshot: snap}
	}
	if !ShowChildPreviewColumn(center, quickViewEnabled, filePreviewEligible) {
		return parent, mid, child, ChildPreviewNone
	}
	if childKind != ChildPreviewDirectoryListing {
		return parent, mid, child, childKind
	}
	if center.CarouselChildPreviewCoalesce {
		if center.CarouselChildCachePaintDuringCoalesce() {
			child = Column{
				Kind:      ColumnChild,
				Populated: true,
				Snapshot:  center.CarouselSideCache.Child,
			}
		}
		return parent, mid, child, childKind
	}
	if snap, ok := center.SnapshotChild(viewportRows); ok {
		child = Column{Kind: ColumnChild, Populated: true, Snapshot: snap}
	}
	return parent, mid, child, childKind
}
