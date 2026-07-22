package panellist

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestTreeConnectorPrefixDepthZero(t *testing.T) {
	th := theme.Default()
	if got := TreeConnectorPrefix(0, true, nil, th); got != "" {
		t.Fatalf("depth 0 prefix = %q, want empty", got)
	}
}

func TestTreeConnectorPrefixDepthOne(t *testing.T) {
	th := theme.Default()
	last := TreeConnectorPrefix(1, true, nil, th)
	if last != th.SymbolTreeEnd()+" " {
		t.Fatalf("last-child depth 1 prefix = %q, want %q", last, th.SymbolTreeEnd()+" ")
	}
	notLast := TreeConnectorPrefix(1, false, nil, th)
	if notLast != th.SymbolTreeBranch()+" " {
		t.Fatalf("non-last depth 1 prefix = %q, want %q", notLast, th.SymbolTreeBranch()+" ")
	}
}

func TestTreeConnectorPrefixNestedMixedAncestors(t *testing.T) {
	th := theme.Default()
	// Depth 3, last child, with ancestor[0] having younger siblings (continue guide) and
	// ancestor[1] not (blank guide).
	got := TreeConnectorPrefix(3, true, []bool{true, false}, th)
	want := th.SymbolTreeContinue() + "  " + "   " + th.SymbolTreeEnd() + " "
	if got != want {
		t.Fatalf("nested prefix = %q, want %q", got, want)
	}
}
