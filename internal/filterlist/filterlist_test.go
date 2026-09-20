package filterlist

import (
	"reflect"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestSyncRanksEmptyQueryPreservesOrderAndSlots(t *testing.T) {
	t.Parallel()
	lines := []string{"alpha", "beta", "gamma"}
	ranked, ranges := SyncRanks(lines, "", len(lines)+2, false)
	if !reflect.DeepEqual(ranked, []int{0, 1, 2}) {
		t.Fatalf("ranked = %v, want 0,1,2", ranked)
	}
	if len(ranges) != 5 {
		t.Fatalf("match-range slots = %d, want 5", len(ranges))
	}
	for i, r := range ranges {
		if len(r) != 0 {
			t.Fatalf("slot %d ranges = %v, want empty", i, r)
		}
	}
}

func TestSyncRanksQuerySlotsRangesByOriginalIndex(t *testing.T) {
	t.Parallel()
	lines := []string{"alpha", "zebra", "alpine"}
	ranked, ranges := SyncRanks(lines, "alp", len(lines), false)
	if !reflect.DeepEqual(ranked, []int{0, 2}) {
		t.Fatalf("ranked = %v, want 0,2", ranked)
	}
	if len(ranges) != 3 {
		t.Fatalf("match-range slots = %d, want 3", len(ranges))
	}
	if len(ranges[0]) == 0 {
		t.Fatal("expected match ranges on original index 0")
	}
	if ranges[1] != nil {
		t.Fatalf("non-match slot 1 = %v, want nil", ranges[1])
	}
	if len(ranges[2]) == 0 {
		t.Fatal("expected match ranges on original index 2")
	}
}

func TestSyncRanksCaseInsensitive(t *testing.T) {
	t.Parallel()
	lines := []string{"Alpha", "beta"}
	ranked, _ := SyncRanks(lines, "alp", len(lines), true)
	if !reflect.DeepEqual(ranked, []int{0}) {
		t.Fatalf("ranked = %v, want [0]", ranked)
	}
	rankedSens, _ := SyncRanks(lines, "alp", len(lines), false)
	if len(rankedSens) != 0 {
		t.Fatalf("case-sensitive ranked = %v, want empty", rankedSens)
	}
}

func TestClampSelection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		selected  int
		rankedLen int
		want      int
	}{
		{name: "empty list", selected: 3, rankedLen: 0, want: 0},
		{name: "past end", selected: 5, rankedLen: 3, want: 2},
		{name: "negative", selected: -2, rankedLen: 4, want: 0},
		{name: "in range", selected: 1, rankedLen: 3, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			selected := tt.selected
			ClampSelection(&selected, tt.rankedLen)
			if selected != tt.want {
				t.Fatalf("selected = %d, want %d", selected, tt.want)
			}
		})
	}
}

func TestHandleSelectionKeyIgnoresWrongFocusAndEmptyList(t *testing.T) {
	t.Parallel()
	selected := 1
	called := false
	ensure := func() { called = true }
	if HandleSelectionKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), 1, &selected, 3, func() int { return 5 }, ensure) {
		t.Fatal("expected focus!=0 to be ignored")
	}
	if HandleSelectionKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), 0, &selected, 0, func() int { return 5 }, ensure) {
		t.Fatal("expected empty ranked list to be ignored")
	}
	if selected != 1 || called {
		t.Fatalf("selected=%d called=%v, want unchanged", selected, called)
	}
}

func TestHandleSelectionKeyPageAndCtrlHomeEnd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		ev          *tcell.EventKey
		start       int
		rows        int
		want        int
		wantHandled bool
	}{
		{name: "up", ev: tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), start: 2, rows: 4, want: 1, wantHandled: true},
		{name: "down", ev: tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), start: 1, rows: 4, want: 2, wantHandled: true},
		{name: "pgup", ev: tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone), start: 5, rows: 4, want: 2, wantHandled: true},
		{name: "pgdn", ev: tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone), start: 0, rows: 4, want: 3, wantHandled: true},
		{name: "ctrl-home", ev: tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModCtrl), start: 4, rows: 4, want: 0, wantHandled: true},
		{name: "ctrl-end", ev: tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModCtrl), start: 1, rows: 4, want: 6, wantHandled: true},
		{name: "home without ctrl", ev: tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone), start: 4, rows: 4, want: 4, wantHandled: false},
		{name: "end without ctrl", ev: tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone), start: 1, rows: 4, want: 1, wantHandled: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			selected := tt.start
			scrolled := 0
			handled := HandleSelectionKey(tt.ev, 0, &selected, 7, func() int { return tt.rows }, func() { scrolled++ })
			if handled != tt.wantHandled {
				t.Fatalf("handled = %v, want %v", handled, tt.wantHandled)
			}
			if selected != tt.want {
				t.Fatalf("selected = %d, want %d", selected, tt.want)
			}
			if tt.wantHandled && scrolled != 1 {
				t.Fatalf("ensureScroll calls = %d, want 1", scrolled)
			}
			if !tt.wantHandled && scrolled != 0 {
				t.Fatalf("ensureScroll calls = %d, want 0", scrolled)
			}
		})
	}
}
