package previewpanel

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func textState(text string, isDiff bool) State {
	return State{Source: SourceExternalANSI, CombinedText: text, IsDiff: isDiff}
}

func TestScrollAnchorsWalkPastDiffNoiseAndShortRows(t *testing.T) {
	st := textState("diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1,3 +1,3 @@\n-removed lantern\n }\n+added meadow\n quiet harbor", true)
	got := st.ScrollAnchors(0, 8, 80, tcell.StyleDefault)
	want := []string{"addedmeadow", "quietharbor"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("ScrollAnchors = %q, want %q", got, want)
	}
}

func TestScrollAnchorsSkipMarkdownSyntaxRows(t *testing.T) {
	st := textState("```\n---\n|---|---|\n## Quiet **Harbor**", false)
	got := st.ScrollAnchors(0, 4, 80, tcell.StyleDefault)
	if len(got) != 1 || got[0] != "quietharbor" {
		t.Fatalf("ScrollAnchors = %q, want only the heading text", got)
	}
}

func TestLocateAnchorMatchesAcrossMarkupAndWrapping(t *testing.T) {
	raw := textState("intro\n\n## Quiet **Harbor** lights over the water", false)
	rendered := textState("intro\n\nQuiet Harbor\nlights over\nthe water", false)
	anchors := raw.ScrollAnchors(2, 1, 80, tcell.StyleDefault)
	if row, ok := rendered.LocateAnchor(anchors, 0, 80, tcell.StyleDefault); !ok || row != 2 {
		t.Fatalf("LocateAnchor = %d %v, want row 2 where the wrapped text begins", row, ok)
	}
}

func TestLocateAnchorIgnoresRemovedDiffLines(t *testing.T) {
	st := textState("@@ -1,2 +1,2 @@\n-quiet harbor\n+loud harbor\n quiet harbor", true)
	row, ok := st.LocateAnchor([]string{"quietharbor"}, 0, 80, tcell.StyleDefault)
	if !ok || row != 3 {
		t.Fatalf("LocateAnchor = %d %v, want the context row 3, not the removed row 1", row, ok)
	}
}

func TestLocateAnchorPicksNearestRepeat(t *testing.T) {
	st := textState("return lantern\nx\nx\nx\nreturn lantern\nx\nx\nx\nreturn lantern", false)
	if row, ok := st.LocateAnchor([]string{"returnlantern"}, 5, 80, tcell.StyleDefault); !ok || row != 4 {
		t.Fatalf("LocateAnchor = %d %v, want the repeat at row 4", row, ok)
	}
}

func TestLocateAnchorFallsToNextAnchor(t *testing.T) {
	st := textState("alpha\nquiet harbor", false)
	row, ok := st.LocateAnchor([]string{"gonefromhere", "quietharbor"}, 0, 80, tcell.StyleDefault)
	if !ok || row != 1 {
		t.Fatalf("LocateAnchor = %d %v, want row 1 from the second anchor", row, ok)
	}
	if _, ok := st.LocateAnchor([]string{"gonefromhere"}, 0, 80, tcell.StyleDefault); ok {
		t.Fatal("LocateAnchor found text the body does not hold")
	}
}
