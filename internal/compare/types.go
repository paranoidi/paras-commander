// Package compare walks two directory trees, hashes file contents, and classifies differences.
package compare

import (
	"fmt"
	"path"
	"strings"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// Kind classifies one compare result row.
type Kind int

const (
	KindEqual Kind = iota
	KindRelocated
	KindPrimaryOnly
	KindSecondaryOnly
	KindContentDiff
	KindSkipped
)

// Phase is the session lifecycle stage.
type Phase int

const (
	PhaseWalking Phase = iota
	PhaseHashing
	PhaseDone
	PhaseError
	PhaseCanceled
)

// FileRecord is one regular file discovered under a compare root.
type FileRecord struct {
	Abs  pathloc.Path
	Rel  string
	Size int64
	// ModTime is the file's modification time in UnixNano (hash-cache key).
	ModTime int64
}

// Row is one aligned primary/secondary compare result.
type Row struct {
	Kind         Kind
	PrimaryRel   string
	SecondaryRel string
	Size         int64
	Hash         [32]byte
	HashDone     bool
	// Hashing is true while a worker is actively reading this row's file(s).
	// Distinct from pending (!HashDone): queued rows stay pending without Hashing.
	Hashing bool
	Err     string
}

// RowKey identifies a row by its two relative paths; it stays stable across rescans as long
// as the pairing does.
type RowKey struct {
	PrimaryRel   string
	SecondaryRel string
}

// Key returns the row's RowKey.
func (r Row) Key() RowKey { return RowKey{PrimaryRel: r.PrimaryRel, SecondaryRel: r.SecondaryRel} }

// MarkHashing sets Hashing on pending rows whose primary and/or secondary rel is
// currently being content-hashed. rows may be mutated in place; the same slice is returned.
func MarkHashing(rows []Row, hashingPrimary, hashingSecondary map[string]struct{}) []Row {
	if len(hashingPrimary) == 0 && len(hashingSecondary) == 0 {
		for i := range rows {
			rows[i].Hashing = false
		}
		return rows
	}
	for i := range rows {
		rows[i].Hashing = false
		if rows[i].HashDone {
			continue
		}
		if rel := rows[i].PrimaryRel; rel != "" {
			if _, ok := hashingPrimary[rel]; ok {
				rows[i].Hashing = true
			}
		}
		if rel := rows[i].SecondaryRel; rel != "" {
			if _, ok := hashingSecondary[rel]; ok {
				rows[i].Hashing = true
			}
		}
	}
	return rows
}

// Snapshot is an immutable compare result generation.
type Snapshot struct {
	PrimaryRoot     pathloc.Path
	SecondaryRoot   pathloc.Path
	Rows            []Row
	Phase           Phase
	WalkedPrimary   int
	WalkedSecondary int
	Hashed          int
	HashTotal       int
	Err             string
}

// Filter is a bitmask of the row kinds visible in the UI. Skipped rows have no bit and stay hidden.
type Filter int

const (
	FilterEqual Filter = 1 << iota
	FilterRelocated
	FilterPrimaryOnly
	FilterSecondaryOnly
	FilterContentDiff

	// FilterNone shows no rows.
	FilterNone Filter = 0
	// FilterAll shows every kind except Skipped.
	FilterAll = FilterEqual | FilterRelocated | FilterPrimaryOnly | FilterSecondaryOnly | FilterContentDiff
)

// filterKinds lists the single-kind filters in display and cycle order.
var filterKinds = []struct {
	Filter Filter
	Kind   Kind
	Label  string
}{
	{FilterEqual, KindEqual, "Equal"},
	{FilterRelocated, KindRelocated, "Relocated"},
	{FilterPrimaryOnly, KindPrimaryOnly, "Left only"},
	{FilterSecondaryOnly, KindSecondaryOnly, "Right only"},
	{FilterContentDiff, KindContentDiff, "Content diff"},
}

// FilterMatches reports whether kind is included in f.
func FilterMatches(f Filter, kind Kind) bool {
	for _, k := range filterKinds {
		if k.Kind == kind {
			return f&k.Filter != 0
		}
	}
	return false
}

// FilterLabel returns a short UI label for f: All, None, or the kind names joined with " + ".
func FilterLabel(f Filter) string {
	switch f & FilterAll {
	case FilterAll:
		return "All"
	case FilterNone:
		return "None"
	}
	var names []string
	for _, k := range filterKinds {
		if f&k.Filter != 0 {
			names = append(names, k.Label)
		}
	}
	return strings.Join(names, " + ")
}

// FilterDialogRadio describes one category checkbox in the compare filter dialog.
type FilterDialogRadio struct {
	Filter   Filter
	Label    string
	Shortcut rune
}

// FilterDialogRadios is the canonical checkbox list for the compare filter dialog and its key handler.
func FilterDialogRadios() []FilterDialogRadio {
	return []FilterDialogRadio{
		{FilterAll, "All", 'A'},
		{FilterEqual, "Equal", 'E'},
		{FilterRelocated, "Relocated", 'R'},
		{FilterPrimaryOnly, "Left only", 'L'},
		{FilterSecondaryOnly, "Right only", 'I'},
		{FilterContentDiff, "Content diff", 'D'},
	}
}

// SelectDialogRadios is the checkbox list for the compare select-by-kind dialog.
// "This side" is the focused column's one-sided kind, "other side" the opposite.
func SelectDialogRadios(focusSecondary bool) []FilterDialogRadio {
	this, other := FilterPrimaryOnly, FilterSecondaryOnly
	if focusSecondary {
		this, other = other, this
	}
	return []FilterDialogRadio{
		{FilterAll, "All", 'A'},
		{FilterEqual, "Equal", 'E'},
		{FilterRelocated, "Relocated", 'R'},
		{FilterContentDiff, "Modified", 'M'},
		{this, "Only this side", 'T'},
		{other, "Only other side", 'N'},
	}
}

// FilterChecked reports whether the checkbox row for bit is checked in mask f.
// FilterAll is the master row: checked only when every kind is on.
func FilterChecked(f, bit Filter) bool {
	if bit == FilterAll {
		return f&FilterAll == FilterAll
	}
	return f&bit != 0
}

// ToggleFilter flips the checkbox row for bit in f. The FilterAll master row clears
// everything when checked and sets everything otherwise.
func ToggleFilter(f, bit Filter) Filter {
	if bit == FilterAll {
		if FilterChecked(f, FilterAll) {
			return FilterNone
		}
		return FilterAll
	}
	return f ^ bit
}

// FilteredRows returns rows from snap that match filter. When ignoreEmpty is true,
// rows with Size == 0 are omitted. match, when non-nil, additionally keeps only rows whose
// basename (PrimaryRel, else SecondaryRel) it accepts.
func FilteredRows(snap Snapshot, filter Filter, ignoreEmpty bool, match func(name string) bool) []Row {
	out := make([]Row, 0, len(snap.Rows))
	for _, row := range snap.Rows {
		if ignoreEmpty && row.Size == 0 {
			continue
		}
		if match != nil && !match(RowBaseName(row)) {
			continue
		}
		if FilterMatches(filter, row.Kind) {
			out = append(out, row)
		}
	}
	return out
}

// RowBaseName is the basename the name filter matches against.
func RowBaseName(row Row) string {
	rel := row.PrimaryRel
	if rel == "" {
		rel = row.SecondaryRel
	}
	return path.Base(rel)
}

// CountEmptyRows returns how many snapshot rows have Size == 0.
func CountEmptyRows(snap Snapshot) int {
	n := 0
	for _, row := range snap.Rows {
		if row.Size == 0 {
			n++
		}
	}
	return n
}

// EndLabel returns the compare view top-right chrome label for the active category
// filter, appending " · N empty hidden" when empties are ignored and at least one
// zero-byte row is hidden, and " · <nameLabel>" when a name filter is active.
func EndLabel(filter Filter, ignoreEmpty bool, snap Snapshot, nameLabel string) string {
	label := FilterLabel(filter)
	if ignoreEmpty {
		if n := CountEmptyRows(snap); n > 0 {
			label = fmt.Sprintf("%s · %d empty hidden", label, n)
		}
	}
	if nameLabel != "" {
		label += " · " + nameLabel
	}
	return label
}

// CycleFilter advances f through All, then each single kind, then back to All.
// Any other mask (none or several kinds) goes to All.
func CycleFilter(f Filter) Filter {
	if f == FilterAll {
		return filterKinds[0].Filter
	}
	for i, k := range filterKinds[:len(filterKinds)-1] {
		if f == k.Filter {
			return filterKinds[i+1].Filter
		}
	}
	return FilterAll
}
