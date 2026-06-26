package compare

import "fmt"

// RowLegend returns a short human-readable description for a compare row kind and glyph.
func RowLegend(kind Kind, glyph string) string {
	switch kind {
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
