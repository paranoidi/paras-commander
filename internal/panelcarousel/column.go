package panelcarousel

import "github.com/paranoidi/paras-commander/internal/panel"

// ColumnKind identifies a carousel pane.
type ColumnKind int

const (
	ColumnParent ColumnKind = iota
	ColumnCenter
	ColumnChild
)

// Column is one carousel pane (parent preview, live center, or child preview).
type Column struct {
	Kind      ColumnKind
	Populated bool
	Active    bool
	Snapshot  panel.ListingSnapshot
}

// ShowChildPreviewColumn reports whether the carousel child (preview) column should be built and painted.
// It is omitted when the center listing has no subdirectories or when Quick view already previews
// the inactive panel (same two-pane layout as the no-subdirectories case).
func ShowChildPreviewColumn(center panel.State, quickViewEnabled bool) bool {
	return center.CarouselCenterHasSubdirectories() && !quickViewEnabled
}

// BuildColumns constructs parent/center/child column descriptors from the live panel state.
// While CarouselChildPreviewCoalesce is set, the child column reuses the last cached snapshot
// and does not read the filesystem; callers paint the child column only after coalesce ends.
func BuildColumns(center panel.State, viewportRows int, quickViewEnabled bool) (parent, mid, child Column) {
	mid = Column{Kind: ColumnCenter, Populated: true, Active: true}
	if snap, ok := center.SnapshotParent(viewportRows); ok {
		parent = Column{Kind: ColumnParent, Populated: true, Snapshot: snap}
	}
	if !ShowChildPreviewColumn(center, quickViewEnabled) {
		return parent, mid, child
	}
	if center.CarouselChildPreviewCoalesce {
		if center.CarouselChildCachePaintDuringCoalesce() {
			child = Column{
				Kind:      ColumnChild,
				Populated: true,
				Snapshot:  center.CarouselSideCache.Child,
			}
		}
		return parent, mid, child
	}
	if snap, ok := center.SnapshotChild(viewportRows); ok {
		child = Column{Kind: ColumnChild, Populated: true, Snapshot: snap}
	}
	return parent, mid, child
}
