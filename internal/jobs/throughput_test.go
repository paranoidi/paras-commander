package jobs

import (
	"testing"
	"time"
)

func TestFormatThroughput(t *testing.T) {
	tests := []struct {
		bps  float64
		want string
	}{
		{0, "—"},
		{-1, "—"},
		{512, "512B/s"},
		{1024, "1KB/s"},
		{1536, "1.5KB/s"},
		{1024 * 1024, "1MB/s"},
		{80 * 1024 * 1024, "80MB/s"},
		{150 * 1024 * 1024, "150MB/s"},
		{1024 * 1024 * 1024, "1GB/s"},
	}
	for _, tt := range tests {
		got := FormatThroughput(tt.bps)
		if got != tt.want {
			t.Fatalf("FormatThroughput(%v) = %q, want %q", tt.bps, got, tt.want)
		}
	}

	// Widest output is "99.9MB/s" (8 cells); the jobs view Speed column relies on that.
	const maxWidth = 8
	for bps := 1.0; bps <= 999*1024*1024*1024; bps *= 1.7 {
		if got := FormatThroughput(bps); len(got) > maxWidth {
			t.Fatalf("FormatThroughput(%v) = %q (len %d), want len <= %d", bps, got, len(got), maxWidth)
		}
	}
}

func TestFormatThroughputWhole(t *testing.T) {
	const mb = 1024 * 1024
	tests := []struct {
		bps  float64
		want string
	}{
		{0, "—"},
		{1536, "2KB/s"},
		{1023.6 * 1024, "1MB/s"},
		{97.2 * mb, "97MB/s"},
		{99.6 * mb, "100MB/s"},
		{150 * mb, "150MB/s"},
	}
	for _, tt := range tests {
		if got := FormatThroughputWhole(tt.bps); got != tt.want {
			t.Fatalf("FormatThroughputWhole(%v) = %q, want %q", tt.bps, got, tt.want)
		}
	}

	// Widest output must fit the menu-bar speed pill's fixed text width (7 cells).
	const maxWidth = 7
	for bps := 1.0; bps <= 999*1024*1024*1024; bps *= 1.7 {
		if got := FormatThroughputWhole(bps); len(got) > maxWidth {
			t.Fatalf("FormatThroughputWhole(%v) = %q (len %d), want len <= %d", bps, got, len(got), maxWidth)
		}
	}
}

func TestEffectiveDisplayThroughputBPS(t *testing.T) {
	started := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	now := started.Add(2 * time.Second)
	if g := EffectiveDisplayThroughputBPS(StatusCompleted, started, now, 1000, 0); g != 0 {
		t.Fatalf("completed got %v want 0", g)
	}
	if g := EffectiveDisplayThroughputBPS(StatusRunning, started, started.Add(500*time.Millisecond), 1000, 0); g != 0 {
		t.Fatalf("elapsed <1s got %v want 0", g)
	}
	if g := EffectiveDisplayThroughputBPS(StatusRunning, started, now, 1000, 0); g != 500 {
		t.Fatalf("lifetime got %v want 500", g)
	}
	if g := EffectiveDisplayThroughputBPS(StatusRunning, started, now, 1000, 999); g != 999 {
		t.Fatalf("prefer DisplaySpeedBPS got %v want 999", g)
	}
}
