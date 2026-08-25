package compare

import "fmt"

// RowPending reports whether the row still awaits content hashing (disk glyph in the UI).
func RowPending(row Row) bool {
	if row.HashDone || row.Kind == KindContentDiff || row.Err != "" {
		return false
	}
	if row.PrimaryRel != "" && row.SecondaryRel != "" && row.PrimaryRel == row.SecondaryRel {
		return true
	}
	return row.Kind == KindPrimaryOnly || row.Kind == KindSecondaryOnly || row.PrimaryRel == "" || row.SecondaryRel == ""
}

// RowLegend returns a short human-readable description for a compare row and glyph.
// Pending / actively hashing rows use those labels instead of the provisional Kind
// (e.g. KindEqual before hashes land must not read as "Identical").
func RowLegend(row Row, glyph string) string {
	if row.Hashing {
		return fmt.Sprintf("%s Hashing", glyph)
	}
	if RowPending(row) {
		return fmt.Sprintf("%s Pending", glyph)
	}
	switch row.Kind {
	case KindEqual:
		return fmt.Sprintf("%s Identical", glyph)
	case KindRelocated:
		return fmt.Sprintf("%s Relocated — same content, different path", glyph)
	case KindPrimaryOnly:
		return fmt.Sprintf("%s Only on primary", glyph)
	case KindSecondaryOnly:
		return fmt.Sprintf("%s Only on secondary", glyph)
	case KindContentDiff:
		return fmt.Sprintf("%s Content differs", glyph)
	case KindSkipped:
		return fmt.Sprintf("%s Skipped", glyph)
	default:
		return glyph
	}
}
