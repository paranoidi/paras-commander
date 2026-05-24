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

// BuildColumns constructs parent/center/child column descriptors from the live panel state.
func BuildColumns(center panel.State, viewportRows int) (parent, mid, child Column) {
	mid = Column{Kind: ColumnCenter, Populated: true, Active: true}
	if snap, ok := center.SnapshotParent(viewportRows); ok {
		parent = Column{Kind: ColumnParent, Populated: true, Snapshot: snap}
	}
	if snap, ok := center.SnapshotChild(viewportRows); ok {
		child = Column{Kind: ColumnChild, Populated: true, Snapshot: snap}
	}
	return parent, mid, child
}
