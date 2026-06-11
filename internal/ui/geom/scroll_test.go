package geom

import "testing"

func TestScrollOffsetEdgePreservesScrollInsideMargin(t *testing.T) {
	const (
		vr     = 20
		total  = 100
		margin = 5
	)
	scroll := 10
	cursor := 15 // pos=5, top=5 bottom=14
	got := ScrollOffsetEdge(cursor, scroll, vr, total, margin)
	if got != scroll {
		t.Fatalf("ScrollOffsetEdge() = %d, want %d (no scroll)", got, scroll)
	}
}

func TestScrollOffsetEdgeScrollsWhenTopMarginViolated(t *testing.T) {
	const (
		vr     = 20
		total  = 100
		margin = 5
	)
	got := ScrollOffsetEdge(14, 10, vr, total, margin)
	want := 9 // cursor 14 at pos 5
	if got != want {
		t.Fatalf("ScrollOffsetEdge() = %d, want %d", got, want)
	}
}

func TestScrollOffsetEdgeScrollsWhenBottomMarginViolated(t *testing.T) {
	const (
		vr     = 20
		total  = 100
		margin = 5
	)
	got := ScrollOffsetEdge(25, 10, vr, total, margin)
	want := 11 // bottom margin 5 at pos 14
	if got != want {
		t.Fatalf("ScrollOffsetEdge() = %d, want %d", got, want)
	}
}

func TestScrollOffsetEdgePinsAtHead(t *testing.T) {
	got := ScrollOffsetEdge(2, 0, 20, 100, 5)
	if got != 0 {
		t.Fatalf("ScrollOffsetEdge() = %d, want 0 at list head", got)
	}
}

func TestScrollOffsetEdgePinsAtTail(t *testing.T) {
	const (
		vr    = 20
		total = 100
	)
	maxOffset := total - vr
	got := ScrollOffsetEdge(total-1, maxOffset, vr, total, 5)
	if got != maxOffset {
		t.Fatalf("ScrollOffsetEdge() = %d, want %d at list tail", got, maxOffset)
	}
}

func TestScrollOffsetEdgeSmallViewportReducesMargin(t *testing.T) {
	// vr=7, margin=5 -> effective margin 3
	got := ScrollOffsetEdge(6, 0, 7, 50, 5)
	want := 3
	if got != want {
		t.Fatalf("ScrollOffsetEdge() = %d, want %d", got, want)
	}
}

func TestScrollOffsetEdgeSinglePage(t *testing.T) {
	got := ScrollOffsetEdge(3, 5, 10, 8, 5)
	if got != 0 {
		t.Fatalf("ScrollOffsetEdge() = %d, want 0 for single-page list", got)
	}
}
