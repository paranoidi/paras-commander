package preview

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

// openFullscreenPreviewSync mirrors OpenFullscreenFilePreviewAt but runs the preview content
// load synchronously (in-goroutine, like TestRunPreviewInternalSetsHighlightedCells does), so
// tests can inspect search matches/scroll immediately without waiting on the background job.
func openFullscreenPreviewSync(t *testing.T, h *Handler, path string) {
	t.Helper()
	h.mu.Lock()
	h.model.FullscreenFilePreview = ui.FilePreviewState{Open: true, Phase: ui.FilePreviewPhasePending, Path: path}
	h.mu.Unlock()
	tw, ch, ok := h.fullscreenFilePreviewLayoutMetrics()
	if !ok {
		t.Fatal("fullscreenFilePreviewLayoutMetrics() ok = false")
	}
	gen := h.previewRunGenFor(previewTargetFullscreen).Add(1)
	req := h.previewRequest(path, tw, ch, filepath.Dir(path), false, nil, previewTargetFullscreen, false)
	h.runPreview(context.Background(), req, previewTargetFullscreen, gen)
}

func writeNumberedLinesFile(t *testing.T, path string, lines int, matchLine int, needle string) {
	t.Helper()
	var b strings.Builder
	for i := 1; i <= lines; i++ {
		if i == matchLine {
			fmt.Fprintf(&b, "%s\n", needle)
			continue
		}
		fmt.Fprintf(&b, "line %d\n", i)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestScrollFilePreviewToSearchMatchCentersMidFileMatch covers the fix where jumping to a
// search match centers it in the viewport instead of pinning it to the top row.
func TestScrollFilePreviewToSearchMatchCentersMidFileMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "haystack.txt")
	writeNumberedLinesFile(t, path, 200, 150, "needle150")

	h, _ := newTestHandler(t, 80, 24)
	openFullscreenPreviewSync(t, h, path)

	h.applyFilePreviewSearchFieldEdit(dialog.FileDialogField{Value: "needle150"})

	h.mu.RLock()
	matches := h.model.FullscreenFilePreview.Search.Matches
	scroll := h.model.FullscreenFilePreview.Scroll
	h.mu.RUnlock()
	if len(matches) != 1 {
		t.Fatalf("Matches = %d, want 1", len(matches))
	}

	tw, ch, lc := h.fullscreenFilePreviewScrollMetrics()
	st := h.model.FullscreenFilePreview
	matchWrappedLine := st.SourceLineToScrollOffset(matches[0].Line, tw, tcell.StyleDefault)
	want := geom.ScrollOffset(matchWrappedLine, ch, lc)
	if scroll != want {
		t.Fatalf("Scroll = %d, want %d (centered)", scroll, want)
	}
	if scroll == matchWrappedLine {
		t.Fatalf("Scroll = %d, want centered offset, not the match pinned to the top row", scroll)
	}
}

// TestScrollFilePreviewToSearchMatchClampsNearEndOfFile covers the existing bottom-edge clamp:
// a match near the end of the file cannot be centered because there is no content below it to
// scroll past, so the viewport stays clamped at the last full screen of content.
func TestScrollFilePreviewToSearchMatchClampsNearEndOfFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "haystack.txt")
	writeNumberedLinesFile(t, path, 200, 199, "needle199")

	h, _ := newTestHandler(t, 80, 24)
	openFullscreenPreviewSync(t, h, path)

	h.applyFilePreviewSearchFieldEdit(dialog.FileDialogField{Value: "needle199"})

	h.mu.RLock()
	scroll := h.model.FullscreenFilePreview.Scroll
	h.mu.RUnlock()

	_, ch, lc := h.fullscreenFilePreviewScrollMetrics()
	want := max(0, lc-ch)
	if scroll != want {
		t.Fatalf("Scroll = %d, want %d (clamped to bottom of file)", scroll, want)
	}
}

// newSearchNavHandler opens a preview of a generated file with three "needle" lines and wires the
// [preview_search] defaults.
func newSearchNavHandler(t *testing.T) *Handler {
	t.Helper()
	path := filepath.Join(t.TempDir(), "haystack.txt")
	var b strings.Builder
	for i := 1; i <= 100; i++ {
		if i%30 == 0 {
			b.WriteString("needle here\n")
		} else {
			fmt.Fprintf(&b, "line %d\n", i)
		}
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	h, _ := newTestHandler(t, 80, 24)
	km, err := keymap.Build(keymap.DefaultFilePreviewSearchOverlayKeys())
	if err != nil {
		t.Fatal(err)
	}
	h.keysFilePreviewSearch = km
	openFullscreenPreviewSync(t, h, path)
	return h
}

func (h *Handler) searchCurrent() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.model.FullscreenFilePreview.Search.Current
}

func TestSearchNavKeysWhileEditing(t *testing.T) {
	h := newSearchNavHandler(t)
	h.startFilePreviewSearch()
	h.applyFilePreviewSearchFieldEdit(dialog.FileDialogField{Value: "needle"})
	if n := len(h.model.FullscreenFilePreview.Search.Matches); n != 3 {
		t.Fatalf("Matches = %d, want 3", n)
	}
	steps := []struct {
		ev   *tcell.EventKey
		want int
	}{
		{tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), 1},
		{tcell.NewEventKey(tcell.KeyCtrlJ, 0, tcell.ModCtrl), 2},
		{tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), 0},
		{tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), 2},
		{tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModCtrl), 1},
	}
	for i, st := range steps {
		h.HandleFilePreviewViewKey(st.ev)
		if got := h.searchCurrent(); got != st.want {
			t.Fatalf("step %d: Current = %d, want %d", i, got, st.want)
		}
	}
	if v := h.model.FullscreenFilePreviewSearchField.Value; v != "needle" {
		t.Fatalf("query field = %q, want unchanged", v)
	}
	if !h.model.FullscreenFilePreview.Search.Editing {
		t.Fatal("search should still be editing")
	}
}

func TestSearchNavKeysAfterAccept(t *testing.T) {
	h := newSearchNavHandler(t)
	h.startFilePreviewSearch()
	h.applyFilePreviewSearchFieldEdit(dialog.FileDialogField{Value: "needle"})
	h.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if h.model.FullscreenFilePreview.Search.Editing {
		t.Fatal("Enter should accept the search")
	}
	h.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if got := h.searchCurrent(); got != 1 {
		t.Fatalf("after Down: Current = %d, want 1", got)
	}
	h.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if got := h.searchCurrent(); got != 0 {
		t.Fatalf("after Up: Current = %d, want 0", got)
	}
}

func TestDownScrollsWithoutSearch(t *testing.T) {
	h := newSearchNavHandler(t)
	h.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if got := h.model.FullscreenFilePreview.Scroll; got != 1 {
		t.Fatalf("Scroll = %d, want 1", got)
	}
}

func TestSearchNavKeyIgnoredWithoutMatches(t *testing.T) {
	h := newSearchNavHandler(t)
	h.startFilePreviewSearch()
	h.applyFilePreviewSearchFieldEdit(dialog.FileDialogField{Value: "zzzzz"})
	if h.handleFilePreviewSearchNavKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)) {
		t.Fatal("Down with no matches should not be consumed")
	}
}
