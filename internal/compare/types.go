// Package compare walks two directory trees, hashes file contents, and classifies differences.
package compare

import (
	"fmt"

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

// Filter selects which row kinds are visible in the UI.
type Filter int

const (
	FilterAll Filter = iota
	FilterEqual
	FilterRelocated
	FilterPrimaryOnly
	FilterSecondaryOnly
	FilterContentDiff
)

// FilterMatches reports whether kind is included in f.
func FilterMatches(f Filter, kind Kind) bool {
	switch f {
	case FilterAll:
		return kind != KindSkipped
	case FilterEqual:
		return kind == KindEqual
	case FilterRelocated:
		return kind == KindRelocated
	case FilterPrimaryOnly:
		return kind == KindPrimaryOnly
	case FilterSecondaryOnly:
		return kind == KindSecondaryOnly
	case FilterContentDiff:
		return kind == KindContentDiff
	default:
		return false
	}
}

// FilterLabel returns a short UI label for f.
func FilterLabel(f Filter) string {
	switch f {
	case FilterAll:
		return "All"
	case FilterEqual:
		return "Equal"
	case FilterRelocated:
		return "Relocated"
	case FilterPrimaryOnly:
		return "Primary only"
	case FilterSecondaryOnly:
		return "Secondary only"
	case FilterContentDiff:
		return "Content diff"
	default:
		return "All"
	}
}

// FilterDialogRadio describes one category radio in the compare filter dialog.
type FilterDialogRadio struct {
	Filter   Filter
	Label    string
	Shortcut rune
}

// FilterDialogRadios is the canonical radio list for the compare filter dialog and its key handler.
func FilterDialogRadios() []FilterDialogRadio {
	return []FilterDialogRadio{
		{FilterAll, "All", 'A'},
		{FilterEqual, "Equal", 'E'},
		{FilterRelocated, "Relocated", 'R'},
		{FilterPrimaryOnly, "Primary only", 'P'},
		{FilterSecondaryOnly, "Secondary only", 'S'},
		{FilterContentDiff, "Content diff", 'D'},
	}
}

// FocusForFilter returns the radio focus index for f (0 when unknown).
func FocusForFilter(f Filter) int {
	for i, r := range FilterDialogRadios() {
		if r.Filter == f {
			return i
		}
	}
	return 0
}

// FilterForFocus maps focus index to a Filter value.
func FilterForFocus(focus int) (Filter, bool) {
	radios := FilterDialogRadios()
	if focus < 0 || focus >= len(radios) {
		return FilterAll, false
	}
	return radios[focus].Filter, true
}

// FilteredRows returns rows from snap that match filter. When ignoreEmpty is true,
// rows with Size == 0 are omitted.
func FilteredRows(snap Snapshot, filter Filter, ignoreEmpty bool) []Row {
	out := make([]Row, 0, len(snap.Rows))
	for _, row := range snap.Rows {
		if ignoreEmpty && row.Size == 0 {
			continue
		}
		if FilterMatches(filter, row.Kind) {
			out = append(out, row)
		}
	}
	return out
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
// zero-byte row is hidden.
func EndLabel(filter Filter, ignoreEmpty bool, snap Snapshot) string {
	label := FilterLabel(filter)
	if !ignoreEmpty {
		return label
	}
	if n := CountEmptyRows(snap); n > 0 {
		return fmt.Sprintf("%s · %d empty hidden", label, n)
	}
	return label
}

// CycleFilter advances f.
func CycleFilter(f Filter) Filter {
	switch f {
	case FilterAll:
		return FilterEqual
	case FilterEqual:
		return FilterRelocated
	case FilterRelocated:
		return FilterPrimaryOnly
	case FilterPrimaryOnly:
		return FilterSecondaryOnly
	case FilterSecondaryOnly:
		return FilterContentDiff
	default:
		return FilterAll
	}
}
