package panellist

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/theme"
)

// TreeConnectorPrefix renders the ancestor guide-line prefix for one tree row, given the
// depth/lastChild/ancestorHasNext shape treeflat.Row already carries. Lifted verbatim from
// dedup's tree renderer (internal/ui/dedup_view_render.go, dedupTreeConnectorPrefix) since the
// algorithm only touches tree shape, not dedup-specific row data.
func TreeConnectorPrefix(depth int, lastChild bool, ancestorHasNext []bool, styles theme.Theme) string {
	if depth == 0 {
		return ""
	}
	var b strings.Builder
	continueGlyph := styles.SymbolTreeContinue()
	branchGlyph := styles.SymbolTreeBranch()
	endGlyph := styles.SymbolTreeEnd()
	for i := range depth - 1 {
		if ancestorHasNext[i] {
			b.WriteString(continueGlyph)
			b.WriteString("  ")
		} else {
			b.WriteString("   ")
		}
	}
	if lastChild {
		b.WriteString(endGlyph)
	} else {
		b.WriteString(branchGlyph)
	}
	b.WriteString(" ")
	return b.String()
}
