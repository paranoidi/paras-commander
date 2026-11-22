package app

import "testing"

func TestPanelFollowReportsOnlyChangedTargets(t *testing.T) {
	var f panelFollow
	a := followTargetOf("/srv/meadow/lantern.txt", false, true)
	b := followTargetOf("/srv/orchard", true, true)
	if a.dir != "/srv/meadow" || a.name != "lantern.txt" || b.dir != "/srv/orchard" || b.name != "" {
		t.Fatalf("targets = %+v %+v", a, b)
	}
	if got, ok := f.next(0, a, b); !ok || got != a {
		t.Fatalf("first call = %+v %v, want the preferred source", got, ok)
	}
	if got, ok := f.next(0, a, b); ok {
		t.Fatalf("nothing changed, got %+v", got)
	}
	// Only the second source moved: it wins even though the first is preferred.
	c := followTargetOf("/srv/willow", true, true)
	if got, ok := f.next(0, a, c); !ok || got != c {
		t.Fatalf("second source moved = %+v %v", got, ok)
	}
	// Both moved: the preferred one wins.
	d := followTargetOf("/srv/reed/sedge.txt", false, true)
	if got, ok := f.next(1, d, followTargetOf("/srv/fern", true, true)); !ok || got.dir != "/srv/fern" {
		t.Fatalf("both moved = %+v %v, want the preferred (second)", got, ok)
	}
	f.reset()
	if _, ok := f.next(0, d); !ok {
		t.Fatal("a reset follows afresh")
	}
	if got, ok := f.next(0, followTargetOf("", false, false)); ok {
		t.Fatalf("no target follows nothing, got %+v", got)
	}
}
