package quickfilter

import "testing"

func TestFilterEngine(t *testing.T) {
	names := []string{"harbor.txt", "lantern.txt", "marble.txt", "lantern-copy.txt"}
	f := Filter{}
	opts := Options{CaseInsensitive: true}
	cur, ok := f.Apply("lant", names, opts)
	if !ok || (cur != 1 && cur != 3) {
		t.Fatalf("Apply = %d,%v", cur, ok)
	}
	if !f.HasMatches() || len(f.Ranges(1)) == 0 || f.Ranges(0) != nil {
		t.Fatalf("ranges wrong")
	}
	if got, _ := f.Cycle(1, 1, opts); got != 3 {
		t.Fatalf("cycle fwd = %d", got)
	}
	if got, _ := f.Cycle(3, 1, opts); got != 1 {
		t.Fatalf("cycle wrap = %d", got)
	}
	if _, ok := f.Apply("zzzz", names, opts); ok {
		t.Fatal("expected no match")
	}
	if got, ok := f.Cycle(2, 1, opts); ok || got != 2 {
		t.Fatal("cycle without matches must report false")
	}
	f.Query, f.Cursor = "ab", 2
	if next, changed := f.Backspace(); !changed || next != "a" {
		t.Fatalf("backspace = %q,%v", next, changed)
	}
	f.Query = "a"
	if got := f.InsertRune('x'); got != "ax" {
		t.Fatalf("insert = %q", got)
	}
}

func TestOptionsFrom(t *testing.T) {
	for in, want := range map[string]bool{"ranked": true, " Ranked ": true, "visual": false, "": false} {
		if got := OptionsFrom(true, in); got.CycleRanked != want || !got.CaseInsensitive {
			t.Fatalf("OptionsFrom(%q) = %+v", in, got)
		}
	}
}
