package preview

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func newThemePickerTestHandler(t *testing.T) (*Handler, *fakeHost) {
	t.Helper()
	h, fh := newTestHandler(t, 80, 24)
	h.model.ViewMode = ui.ViewFilePreview
	h.mu.Lock()
	h.model.FullscreenFilePreview = ui.FilePreviewState{
		Open:         true,
		Path:         "/tmp/alpha.txt",
		Phase:        ui.FilePreviewPhaseDone,
		CombinedText: "preview\n",
	}
	h.mu.Unlock()
	return h, fh
}

func TestSyncFilePreviewThemePickerRanksFiltersLabels(t *testing.T) {
	h, _ := newThemePickerTestHandler(t)
	h.model.FilePreviewThemePicker = dialog.FilePreviewThemePickerState{
		Open: true,
		Choices: []dialog.ThemeChoice{
			{Name: "monokai", Label: "monokai"},
			{Name: "github", Label: "github"},
		},
		DisplayLines: []string{"monokai", "github"},
		Query:        "git",
	}
	h.syncFilePreviewThemePickerRanks()
	if len(h.model.FilePreviewThemePicker.Ranked) != 1 {
		t.Fatalf("ranked len = %d, want 1 match for query test", len(h.model.FilePreviewThemePicker.Ranked))
	}
	if idx := h.model.FilePreviewThemePicker.Ranked[0]; idx != 1 {
		t.Fatalf("ranked[0] = %d, want index 1 (github)", idx)
	}
}

func TestOpenFilePreviewThemePickerPopulatesChromaStyles(t *testing.T) {
	h, _ := newThemePickerTestHandler(t)
	h.openFilePreviewThemePicker()
	st := h.model.FilePreviewThemePicker
	if len(st.Choices) < 10 {
		t.Fatalf("Choices len=%d, want many Chroma styles", len(st.Choices))
	}
	if len(st.DisplayLines) != len(st.Choices) {
		t.Fatalf("DisplayLines len=%d Choices len=%d", len(st.DisplayLines), len(st.Choices))
	}
	for i, c := range st.Choices {
		if c.Name == "" || c.Label != c.Name {
			t.Fatalf("choice[%d] = %+v, want Name==Label", i, c)
		}
		if c.Name == "default" {
			t.Fatal("app UI theme name default must not appear in picker")
		}
		if st.DisplayLines[i] != c.Label {
			t.Fatalf("DisplayLines[%d]=%q want Label %q", i, st.DisplayLines[i], c.Label)
		}
	}
}

func TestPreviewStylePickerDebounceDefersRefreshUntilFlush(t *testing.T) {
	h, fh := newThemePickerTestHandler(t)
	fh.cfg.UI.KeyRepeatDebounceMS = 500
	h.openFilePreviewThemePicker()

	genAfterOpen := h.filePreviewRunGen.Load()
	styleAfterOpen := fh.cfg.Preview.Style
	h.handleFilePreviewThemePickerKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))

	if h.filePreviewRunGen.Load() != genAfterOpen {
		t.Fatal("debounced style change should not start preview immediately")
	}
	// Border and content are both gated by the debounce: style unchanged until flush.
	if fh.cfg.Preview.Style != styleAfterOpen {
		t.Fatalf("preview.style changed to %q before flush, want unchanged %q", fh.cfg.Preview.Style, styleAfterOpen)
	}
	if !h.FlushStylePickerPreviewNow() {
		t.Fatal("FlushStylePickerPreviewNow should run deferred preview")
	}
	if h.filePreviewRunGen.Load() == genAfterOpen {
		t.Fatal("flush should start preview refresh")
	}
	if fh.cfg.Preview.Style == styleAfterOpen {
		t.Fatalf("preview.style still %q after flush, want new selection", styleAfterOpen)
	}
}
