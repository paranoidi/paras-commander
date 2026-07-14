package previewpanel_test

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func TestFindMatchesCaseInsensitiveNonOverlapping(t *testing.T) {
	st := previewpanel.State{
		Source:       previewpanel.SourceExternalANSI,
		CombinedText: "Foo foo\nFOOBAR baz",
	}
	matches := st.FindMatches("foo")
	want := []previewpanel.SearchMatch{
		{Start: 0, End: 3, Line: 0},
		{Start: 4, End: 7, Line: 0},
		{Start: 8, End: 11, Line: 1},
	}
	if len(matches) != len(want) {
		t.Fatalf("matches = %+v, want %+v", matches, want)
	}
	for i, m := range matches {
		if m != want[i] {
			t.Fatalf("match[%d] = %+v, want %+v", i, m, want[i])
		}
	}
}

func TestFindMatchesEmptyQuery(t *testing.T) {
	st := previewpanel.State{Source: previewpanel.SourceExternalANSI, CombinedText: "abc"}
	if matches := st.FindMatches(""); matches != nil {
		t.Fatalf("FindMatches(\"\") = %+v, want nil", matches)
	}
}

func TestFindMatchesNoOccurrence(t *testing.T) {
	st := previewpanel.State{Source: previewpanel.SourceExternalANSI, CombinedText: "abc def"}
	if matches := st.FindMatches("xyz"); matches != nil {
		t.Fatalf("FindMatches(no match) = %+v, want nil", matches)
	}
}
