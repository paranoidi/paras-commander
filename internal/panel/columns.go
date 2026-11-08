package panel

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/panellist"
)

// ColumnID names one file column a panel can show after the name and meta columns.
type ColumnID uint8

const (
	ColumnSize ColumnID = iota
	ColumnModified
	ColumnChanged
	ColumnAccessed
	ColumnCreated
	ColumnPermissions
	ColumnOwner
	ColumnGroup
	ColumnLinks
)

// ColumnTimeCells is the width of a timestamp column ("2006-01-02 15:04").
const ColumnTimeCells = 16

// columnGap is the blank cells before a file column other than size (panellist.SizeGap).
const columnGap = 2

// ColumnSpec describes one file column. Columns() is the single source for its name, title, width
// and mnemonic, and its position is both the drawn order and the dialog order.
type ColumnSpec struct {
	ID ColumnID
	// Name is the column's name in column-profiles.toml.
	Name  string
	Title string
	Width int
	// Gap is the blank cells before the column.
	Gap        int
	RightAlign bool
	// Mnemonic is its checkbox key in the Columns dialog.
	Mnemonic rune
	// SortMode is what a header click sorts by; SortName, the zero value, means the column does
	// not sort (no file column is the name).
	SortMode SortMode
}

// Sortable reports whether a header click on the column sorts.
func (s ColumnSpec) Sortable() bool { return s.SortMode != SortName }

var columnSpecs = []ColumnSpec{
	{ID: ColumnSize, Name: "size", Title: "Size", Width: panellist.SizeCells, Gap: panellist.SizeGap, RightAlign: true, Mnemonic: 's', SortMode: SortSize},
	{ID: ColumnModified, Name: "modified", Title: "Modified", Width: ColumnTimeCells, Gap: columnGap, Mnemonic: 'm', SortMode: SortMtime},
	{ID: ColumnChanged, Name: "changed", Title: "Changed", Width: ColumnTimeCells, Gap: columnGap, Mnemonic: 'h'},
	{ID: ColumnAccessed, Name: "accessed", Title: "Accessed", Width: ColumnTimeCells, Gap: columnGap, Mnemonic: 'a'},
	{ID: ColumnCreated, Name: "created", Title: "Created", Width: ColumnTimeCells, Gap: columnGap, Mnemonic: 'r'},
	{ID: ColumnPermissions, Name: "permissions", Title: "Permissions", Width: 11, Gap: columnGap, Mnemonic: 'p'},
	{ID: ColumnOwner, Name: "owner", Title: "Owner", Width: 10, Gap: columnGap, Mnemonic: 'w'},
	{ID: ColumnGroup, Name: "group", Title: "Group", Width: 10, Gap: columnGap, Mnemonic: 'g'},
	{ID: ColumnLinks, Name: "links", Title: "Links", Width: 5, Gap: columnGap, RightAlign: true, Mnemonic: 'n'},
}

// Columns returns every file column in drawn order. Callers must not modify it.
func Columns() []ColumnSpec { return columnSpecs }

// ColumnSet is the set of file columns a panel shows.
type ColumnSet uint16

// Column sets of the built-in profiles.
const (
	ColumnsBrief       = ColumnSet(1 << ColumnSize)
	ColumnsModified    = ColumnSet(1<<ColumnSize | 1<<ColumnModified)
	ColumnsPermissions = ColumnSet(1<<ColumnSize | 1<<ColumnPermissions)
)

// Has reports whether the set shows id.
func (c ColumnSet) Has(id ColumnID) bool { return c&(1<<id) != 0 }

// With returns the set with id added.
func (c ColumnSet) With(id ColumnID) ColumnSet { return c | 1<<id }

// Without returns the set with id removed.
func (c ColumnSet) Without(id ColumnID) ColumnSet { return c &^ (1 << id) }

// Specs returns the columns of the set in drawn order.
func (c ColumnSet) Specs() []ColumnSpec {
	out := make([]ColumnSpec, 0, len(columnSpecs))
	for _, spec := range columnSpecs {
		if c.Has(spec.ID) {
			out = append(out, spec)
		}
	}
	return out
}

// DropRightmost returns the set without its last drawn column.
func (c ColumnSet) DropRightmost() ColumnSet {
	for i := len(columnSpecs) - 1; i >= 0; i-- {
		if id := columnSpecs[i].ID; c.Has(id) {
			return c.Without(id)
		}
	}
	return c
}

// ReservedWidth is the cells the set takes after the name (and meta) column, gaps included.
func (c ColumnSet) ReservedWidth() int {
	total := 0
	for _, spec := range columnSpecs {
		if c.Has(spec.ID) {
			total += spec.Width + spec.Gap
		}
	}
	return total
}

// Names returns the set's profile names in drawn order; never nil, so an empty set saves as [].
func (c ColumnSet) Names() []string {
	names := []string{}
	for _, spec := range c.Specs() {
		names = append(names, spec.Name)
	}
	return names
}

// ParseColumnNames builds a set from profile names. Unknown names are skipped and returned, so a
// hand-edited profile still applies what it can.
func ParseColumnNames(names []string) (ColumnSet, []string) {
	var set ColumnSet
	var unknown []string
nameLoop:
	for _, name := range names {
		for _, spec := range columnSpecs {
			if spec.Name == name {
				set = set.With(spec.ID)
				continue nameLoop
			}
		}
		unknown = append(unknown, name)
	}
	return set, unknown
}

// NameColumnTitle is the name column's header, with the sort arrow when the name is the sort key.
func (s State) NameColumnTitle(showIcons bool) string {
	if s.primarySortUsesDiskTotals() {
		return listNameColumnTitle(showIcons, 0)
	}
	switch s.Sort.Mode {
	case SortMeta, SortSize, SortMtime:
		return listNameColumnTitle(showIcons, 0)
	default:
		return listNameColumnTitle(showIcons, SortArrow(!s.Sort.Reverse))
	}
}

// columnSortArrow is the sort arrow a file column carries, 0 when it is not the sort key. A
// disk-totals sort orders by size whatever the sort mode says.
func (s State) columnSortArrow(spec ColumnSpec) rune {
	switch {
	case s.primarySortUsesDiskTotals():
		if spec.ID == ColumnSize {
			return SortArrow(false)
		}
		return 0
	case spec.Sortable() && s.Sort.Mode == spec.SortMode:
		return SortArrow(!s.Sort.Reverse)
	default:
		return 0
	}
}

// ColumnTitle is the header of a file column, with the sort arrow when it is the sort key. It does
// not depend on the panel showing the column: the carousel draws its size column regardless.
func (s State) ColumnTitle(spec ColumnSpec) string {
	if arrow := s.columnSortArrow(spec); arrow != 0 {
		return fmt.Sprintf("%c%s", arrow, spec.Title)
	}
	return spec.Title
}

// SizeColumnTitle is ColumnTitle of the size column.
func (s State) SizeColumnTitle() string { return s.ColumnTitle(columnSpecs[0]) }

// SortedColumnTitle is the one title of a name-only header: the shown file column being sorted on
// when there is one, else the name column.
func (s State) SortedColumnTitle(showIcons bool) string {
	for _, spec := range s.Columns.Specs() {
		if s.columnSortArrow(spec) != 0 {
			return s.ColumnTitle(spec)
		}
	}
	return s.NameColumnTitle(showIcons)
}
