package app

import (
	"testing"
	"time"
)

// TestDevMoveBarDemoStripRemovalPhases checks the simulated move fills over the whole run and is
// removal-coloured only in the last half second of every three, with the layout fields constant.
func TestDevMoveBarDemoStripRemovalPhases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		elapsed  time.Duration
		removing bool
	}{
		{0, false},
		{2400 * time.Millisecond, false},
		{2600 * time.Millisecond, true},
		{3 * time.Second, false},
		{5900 * time.Millisecond, true},
		{7 * time.Second, false},
	}
	for _, tc := range tests {
		s := devMoveBarDemoStrip(tc.elapsed)
		if s.Removing != tc.removing || s.Deleting {
			t.Fatalf("at %v Removing = %v (Deleting %v), want %v", tc.elapsed, s.Removing, s.Deleting, tc.removing)
		}
		if want := float64(tc.elapsed) / float64(devMoveBarDemoTotal); s.ProgressFrac != want {
			t.Fatalf("at %v ProgressFrac = %v, want %v", tc.elapsed, s.ProgressFrac, want)
		}
		if !s.SpeedSlot || s.Speed == "" || len(s.Groups) != 1 {
			t.Fatalf("at %v strip = %+v; want speed slot, speed and one group throughout", tc.elapsed, s)
		}
	}
	if s := devMoveBarDemoStrip(20 * time.Second); s.ProgressFrac != 1 {
		t.Fatalf("past the end ProgressFrac = %v, want 1", s.ProgressFrac)
	}
}
