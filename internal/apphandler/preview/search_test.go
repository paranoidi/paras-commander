package preview

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
func newSearchNavHandler(t *testing.T) (*Handler, *fakeHost) {
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
	h, fh := newTestHandler(t, 80, 24)
	km, err := keymap.Build(keymap.DefaultFilePreviewSearchOverlayKeys())
	if err != nil {
		t.Fatal(err)
	}
	h.keysFilePreviewSearch = km
	openFullscreenPreviewSync(t, h, path)
	return h, fh
}

func (h *Handler) searchCurrent() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.model.FullscreenFilePreview.Search.Current
}

func TestSearchNavKeysWhileEditing(t *testing.T) {
	h, fh := newSearchNavHandler(t)
	h.startFilePreviewSearch()
	h.applyFilePreviewSearchFieldEdit(dialog.FileDialogField{Value: "needle"})
	if n := len(h.model.FullscreenFilePreview.Search.Matches); n != 3 {
		t.Fatalf("Matches = %d, want 3", n)
	}
	const below, above = "Search: no more matches below", "Search: no more matches above"
	key := func(k tcell.Key, m tcell.ModMask) *tcell.EventKey { return tcell.NewEventKey(k, 0, m) }
	down, ctrlJ := key(tcell.KeyDown, tcell.ModNone), key(tcell.KeyCtrlJ, tcell.ModCtrl)
	up, ctrlK := key(tcell.KeyUp, tcell.ModNone), key(tcell.KeyCtrlK, tcell.ModCtrl)
	steps := []struct {
		ev   *tcell.EventKey
		want int
		msgs []string
	}{
		{down, 1, nil},
		{ctrlJ, 2, nil},
		{down, 2, []string{below}},
		{ctrlJ, 2, []string{below, below}},
		{up, 1, []string{below, below}},
		{ctrlK, 0, []string{below, below}},
		{up, 0, []string{below, below, above}},
		{ctrlK, 0, []string{below, below, above, above}},
	}
	for i, st := range steps {
		if !h.handleFilePreviewSearchNavKey(st.ev) {
			t.Fatalf("step %d: key not handled", i)
		}
		if got := h.searchCurrent(); got != st.want {
			t.Fatalf("step %d: Current = %d, want %d", i, got, st.want)
		}
		if !slices.Equal(fh.messages, st.msgs) {
			t.Fatalf("step %d: messages = %q, want %q", i, fh.messages, st.msgs)
		}
	}
	if v := h.model.FullscreenFilePreviewSearchField.Value; v != "needle" {
		t.Fatalf("query field = %q, want unchanged", v)
	}
	if !h.model.FullscreenFilePreview.Search.Editing {
		t.Fatal("search should still be editing")
	}
}

func TestSearchNavKeysDoNotJumpAfterAccept(t *testing.T) {
	h, _ := newSearchNavHandler(t)
	h.startFilePreviewSearch()
	h.applyFilePreviewSearchFieldEdit(dialog.FileDialogField{Value: "needle"})
	h.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if h.model.FullscreenFilePreview.Search.Editing {
		t.Fatal("Enter should accept the search")
	}
	cur := h.searchCurrent()
	scroll := h.model.FullscreenFilePreview.Scroll
	h.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if got := h.searchCurrent(); got != cur {
		t.Fatalf("Current = %d, want unchanged %d", got, cur)
	}
	if got := h.model.FullscreenFilePreview.Scroll; got != scroll+1 {
		t.Fatalf("Scroll = %d, want %d", got, scroll+1)
	}
}

func TestDownScrollsWithoutSearch(t *testing.T) {
	h, _ := newSearchNavHandler(t)
	h.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if got := h.model.FullscreenFilePreview.Scroll; got != 1 {
		t.Fatalf("Scroll = %d, want 1", got)
	}
}

func TestSearchNavKeyIgnoredWithoutMatches(t *testing.T) {
	h, _ := newSearchNavHandler(t)
	h.startFilePreviewSearch()
	h.applyFilePreviewSearchFieldEdit(dialog.FileDialogField{Value: "zzzzz"})
	if h.handleFilePreviewSearchNavKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)) {
		t.Fatal("Down with no matches should not be consumed")
	}
}
