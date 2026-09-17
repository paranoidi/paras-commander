package compare

import "fmt"

// RowPending reports whether the row still awaits content hashing (disk icon in the UI).
func RowPending(row Row) bool {
	if row.HashDone || row.Kind == KindContentDiff || row.Err != "" {
		return false
	}
	if row.PrimaryRel != "" && row.SecondaryRel != "" && row.PrimaryRel == row.SecondaryRel {
		return true
	}
	return row.Kind == KindPrimaryOnly || row.Kind == KindSecondaryOnly || row.PrimaryRel == "" || row.SecondaryRel == ""
}

// RowLegend returns a short human-readable description for a compare row and icon.
// Pending / actively hashing rows use those labels instead of the provisional Kind
// (e.g. KindEqual before hashes land must not read as "Identical").
func RowLegend(row Row, icon string) string {
	if row.Hashing {
		return fmt.Sprintf("%s Hashing", icon)
	}
	if RowPending(row) {
		return fmt.Sprintf("%s Pending", icon)
	}
	switch row.Kind {
	case KindEqual:
		return fmt.Sprintf("%s Identical", icon)
	case KindRelocated:
		return fmt.Sprintf("%s Relocated — same content, different path", icon)
	case KindPrimaryOnly:
		return fmt.Sprintf("%s Only on primary", icon)
	case KindSecondaryOnly:
		return fmt.Sprintf("%s Only on secondary", icon)
	case KindContentDiff:
		return fmt.Sprintf("%s Content differs", icon)
	case KindSkipped:
		return fmt.Sprintf("%s Skipped", icon)
	default:
		return icon
	}
}
