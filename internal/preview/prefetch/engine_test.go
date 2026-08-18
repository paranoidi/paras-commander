package prefetch

import (
	"sync"
	"testing"
)

func TestPriorityGroupPrioritizesDirectionOfTravel(t *testing.T) {
	cases := []struct {
		name   string
		offset int
		dir    int
		want   int
	}{
		{"caret entry always first", 0, 1, 0},
		{"caret entry first even with no dir", 0, 0, 0},
		{"no dir bias: ahead same group as behind", 3, 0, 1},
		{"no dir bias: behind same group as ahead", -3, 0, 1},
		{"forward dir: ahead prioritized", 2, 1, 1},
		{"forward dir: behind deprioritized", -2, 1, 2},
		{"backward dir: behind prioritized", -2, -1, 1},
		{"backward dir: ahead deprioritized", 2, -1, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := priorityGroup(c.offset, c.dir); got != c.want {
				t.Errorf("priorityGroup(%d, %d) = %d, want %d", c.offset, c.dir, got, c.want)
			}
		})
	}
}

func TestScheduleOrdersPendingByDirectionThenDistance(t *testing.T) {
	e := &Engine{cache: NewCache(1<<20, 0, "")}
	e.cond = sync.NewCond(&e.mu)
	items := []Item{
		{Path: "far-behind", Kind: KindImage, Offset: -3},
		{Path: "near-ahead", Kind: KindImage, Offset: 1},
		{Path: "caret", Kind: KindImage, Offset: 0},
		{Path: "near-behind", Kind: KindImage, Offset: -1},
		{Path: "far-ahead", Kind: KindImage, Offset: 3},
	}
	e.Schedule(items, 1) // caret moving forward
	want := []string{"caret", "near-ahead", "far-ahead", "near-behind", "far-behind"}
	if len(e.pending) != len(want) {
		t.Fatalf("pending len = %d, want %d", len(e.pending), len(want))
	}
	for i, p := range want {
		if e.pending[i].Path != p {
			t.Errorf("pending[%d] = %q, want %q", i, e.pending[i].Path, p)
		}
	}
}
