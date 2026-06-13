package uiscrollbar

import "testing"

func TestComputeMetrics_noScrollNeeded(t *testing.T) {
	t.Parallel()
	_, ok := ComputeMetrics(5, 10, 0)
	if ok {
		t.Fatal("expected ok=false when total <= visible")
	}
}

func TestComputeMetrics_fitsExactly(t *testing.T) {
	t.Parallel()
	_, ok := ComputeMetrics(8, 8, 0)
	if ok {
		t.Fatal("expected ok=false when total == visible")
	}
}

func TestComputeMetrics_thumbDotAtTop(t *testing.T) {
	t.Parallel()
	m, ok := ComputeMetrics(100, 10, 0)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if m.ThumbDotRow != 0 {
		t.Fatalf("ThumbDotRow = %d, want 0", m.ThumbDotRow)
	}
}

func TestComputeMetrics_thumbAtTop(t *testing.T) {
	t.Parallel()
	m, ok := ComputeMetrics(100, 10, 0)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if m.ThumbStart != 0 {
		t.Fatalf("ThumbStart = %d, want 0", m.ThumbStart)
	}
	if m.ThumbSize < 1 {
		t.Fatalf("ThumbSize = %d, want >= 1", m.ThumbSize)
	}
}

func TestComputeMetrics_thumbAtBottom(t *testing.T) {
	t.Parallel()
	m, ok := ComputeMetrics(100, 10, 90)
	if !ok {
		t.Fatal("expected ok=true")
	}
	wantStart := 10 - m.ThumbSize
	if m.ThumbStart != wantStart {
		t.Fatalf("ThumbStart = %d, want %d", m.ThumbStart, wantStart)
	}
}

func TestComputeMetrics_thumbDotAtBottom(t *testing.T) {
	t.Parallel()
	m, ok := ComputeMetrics(100, 10, 90)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if m.ThumbDotRow != 9 {
		t.Fatalf("ThumbDotRow = %d, want 9", m.ThumbDotRow)
	}
}

func TestComputeMetrics_clampsOffset(t *testing.T) {
	t.Parallel()
	m, ok := ComputeMetrics(20, 5, 999)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if m.Offset != 15 {
		t.Fatalf("Offset = %d, want 15", m.Offset)
	}
}
