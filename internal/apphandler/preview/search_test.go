package preview

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
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
	gen := h.filePreviewRunGen.Add(1)
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
