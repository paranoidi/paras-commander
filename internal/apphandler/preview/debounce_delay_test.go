package preview

import (
	"testing"
	"time"
)

// previewDebounceDelay must pick the image/media debounce for targets that go through the
// terminal graphics path and the key-repeat debounce for everything else.
func TestPreviewDebounceDelayPicksImageDelay(t *testing.T) {
	h, fh := newTestHandler(t, 120, 30)
	fh.cfg.UI.KeyRepeatDebounceMS = 45
	fh.cfg.UI.ImagePreviewDebounceMS = 500

	cases := []struct {
		path string
		want time.Duration
	}{
		{"/harbor/lantern/meadow.png", 500 * time.Millisecond},
		{"/harbor/lantern/THISTLE.JPEG", 500 * time.Millisecond},
		{"/harbor/lantern/quarry.mp4", 500 * time.Millisecond},
		{"/harbor/lantern/willow.flac", 500 * time.Millisecond},
		{"/harbor/lantern/cobble.txt", 45 * time.Millisecond},
		{"/harbor/lantern/pennant", 45 * time.Millisecond},
		{"", 45 * time.Millisecond},
	}
	for _, tc := range cases {
		if got := h.previewDebounceDelay(tc.path); got != tc.want {
			t.Errorf("previewDebounceDelay(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
