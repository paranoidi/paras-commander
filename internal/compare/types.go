// Package compare walks two directory trees, hashes file contents, and classifies differences.
package compare

import (
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
	Err          string
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

// FilteredRows returns rows from snap that match filter.
func FilteredRows(snap Snapshot, filter Filter) []Row {
	out := make([]Row, 0, len(snap.Rows))
	for _, row := range snap.Rows {
		if FilterMatches(filter, row.Kind) {
			out = append(out, row)
		}
	}
	return out
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
