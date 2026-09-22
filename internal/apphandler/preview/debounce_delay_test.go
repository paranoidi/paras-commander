package preview

import (
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
)

// quickViewDebounceDelay must pick PreviewDebounceMS for every target that runs a preview (any
// file, a directory served by a [[preview.commands]] rule) and KeyRepeatDebounceMS for directory
// listings and plain messages. Paths are synthetic: the decision must come from the listing entry
// type alone, never from a stat on the UI goroutine.
func TestQuickViewDebounceDelayPicksPreviewDelay(t *testing.T) {
	h, fh := newTestHandler(t, 120, 30)
	fh.cfg.UI.KeyRepeatDebounceMS = 45
	fh.cfg.UI.PreviewDebounceMS = 500
	fh.cfg.Preview.Commands = []config.PreviewCommandRule{
		{When: []string{"t d & d ^/harbor/movies(|/$)"}, Command: "movie-info %f"},
	}
	fh.syncFollowTargetPath = func(p *panel.State) (string, bool) {
		e, ok := p.CurrentEntry()
		return e.Path, ok
	}

	cases := []struct {
		name  string
		entry localfs.Entry
		want  time.Duration
	}{
		{"image file", localfs.Entry{Name: "meadow.png", Path: "/harbor/lantern/meadow.png", Type: localfs.EntryFile, Size: 9}, 500 * time.Millisecond},
		{"text file", localfs.Entry{Name: "cobble.txt", Path: "/harbor/lantern/cobble.txt", Type: localfs.EntryFile, Size: 9}, 500 * time.Millisecond},
		{"empty file (message)", localfs.Entry{Name: "pennant", Path: "/harbor/lantern/pennant", Type: localfs.EntryFile}, 45 * time.Millisecond},
		{"directory listing", localfs.Entry{Name: "quarry", Path: "/harbor/lantern/quarry", Type: localfs.EntryDirectory}, 45 * time.Millisecond},
		{"directory served by a rule", localfs.Entry{Name: "thistle", Path: "/harbor/movies/thistle", Type: localfs.EntryDirectory}, 500 * time.Millisecond},
	}
	for _, tc := range cases {
		h.model.Primary = panel.State{Entries: []localfs.Entry{tc.entry}}
		if got := h.quickViewDebounceDelay(); got != tc.want {
			t.Errorf("%s: quickViewDebounceDelay() = %v, want %v", tc.name, got, tc.want)
		}
	}
	h.model.Primary = panel.State{}
	if got := h.quickViewDebounceDelay(); got != 45*time.Millisecond {
		t.Errorf("no entry: quickViewDebounceDelay() = %v, want 45ms", got)
	}
}
