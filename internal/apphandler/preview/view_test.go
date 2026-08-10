package preview

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func writeFileForPreviewView(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFullscreenPreviewTextWidthReservesScrollbarGutterForPlainContent(t *testing.T) {
	dir := t.TempDir()
	writeFileForPreviewView(t, filepath.Join(dir, "a.txt"))

	h, _ := newTestHandler(t, 80, 20)
	if err := h.OpenFullscreenFilePreviewAt(filepath.Join(dir, "a.txt")); err != nil {
		t.Fatalf("OpenFullscreenFilePreviewAt: %v", err)
	}

	union, ok := h.fullscreenPreviewUnionRect()
	if !ok {
		t.Fatal("fullscreenPreviewUnionRect() ok = false")
	}
	previewRect, _ := ui.SplitFullscreenPreviewRects(union, h.model.FilePreviewThemePicker.Open, h.model.FilePreviewThemePicker.Choices)

	tw, ok := h.fullscreenPreviewTextWidth()
	if !ok {
		t.Fatal("fullscreenPreviewTextWidth() ok = false")
	}
	if want := previewRect.Width - 1; tw != want {
		t.Fatalf("fullscreenPreviewTextWidth() = %d, want %d (previewRect.Width-1, 1-col scrollbar gutter)", tw, want)
	}
}

func TestFilePreviewRunGenStaleSkipsRunningPatch(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	writeFileForPreviewView(t, path)
	h, _ := newTestHandler(t, 80, 24)

	h.mu.Lock()
	h.model.FilePreview.Open = true
	h.model.FilePreview.Phase = ui.FilePreviewPhasePending
	h.model.FilePreview.Path = path
	h.mu.Unlock()
	staleGen := h.filePreviewRunGen.Add(1)
	h.filePreviewRunGen.Add(1)

	h.runPreview(context.Background(), h.previewRequest(path, 80, 20, root, false, nil, previewTargetInactive), previewTargetInactive, staleGen)

	h.mu.RLock()
	ph := h.model.FilePreview.Phase
	h.mu.RUnlock()
	if ph != ui.FilePreviewPhasePending {
		t.Fatalf("Phase = %v, want Pending when run gen is stale at start", ph)
	}
}

func TestRunPreviewInternalSetsHighlightedCells(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.go")
	writeFileForPreviewView(t, path)
	h, fh := newTestHandler(t, 80, 24)
	fh.cfg.Preview.Mode = config.PreviewModeInternal
	fh.cfg.Preview.LineNumbers = true

	h.mu.Lock()
	h.model.FilePreview.Open = true
	h.model.FilePreview.Phase = ui.FilePreviewPhasePending
	h.model.FilePreview.Path = path
	h.mu.Unlock()
	gen := h.filePreviewRunGen.Add(1)
	h.runPreview(context.Background(), h.previewRequest(path, 80, 20, root, false, nil, previewTargetInactive), previewTargetInactive, gen)

	h.mu.RLock()
	st := h.model.FilePreview
	h.mu.RUnlock()
	if st.Phase != ui.FilePreviewPhaseDone {
		t.Fatalf("Phase = %v, want Done", st.Phase)
	}
	if st.Source != ui.PreviewSourceInternalHighlighted {
		t.Fatalf("Source = %v, want internal highlighted", st.Source)
	}
	if len(st.HighlightedCells) == 0 {
		t.Fatal("HighlightedCells empty, want Chroma output")
	}
}

func TestFileViewCloseActionReturnsToBrowserNormally(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	writeFileForPreviewView(t, path)
	h, _ := newTestHandler(t, 80, 20)
	if err := h.OpenFullscreenFilePreviewAt(path); err != nil {
		t.Fatalf("OpenFullscreenFilePreviewAt: %v", err)
	}

	quit, handled := h.tryFilePreviewAction(keymap.ActionFileViewClose)
	if !handled {
		t.Fatal("handled = false, want true")
	}
	if quit {
		t.Fatal("quit = true, want false when not launched as a standalone file viewer")
	}
	if h.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want ViewBrowser", h.model.ViewMode)
	}
}

func TestFileViewCloseActionQuitsWhenLaunchedAsFileViewer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	writeFileForPreviewView(t, path)
	h, fh := newTestHandler(t, 80, 20)
	fh.launchedAsFileViewer = true
	if err := h.OpenFullscreenFilePreviewAt(path); err != nil {
		t.Fatalf("OpenFullscreenFilePreviewAt: %v", err)
	}

	quit, handled := h.tryFilePreviewAction(keymap.ActionFileViewClose)
	if !handled {
		t.Fatal("handled = false, want true")
	}
	if !quit {
		t.Fatal("quit = false, want true when launched as a standalone file viewer")
	}
}

// TestRefreshPreviewTargetAfterResizeReRunsOnlyOnWidthChange covers the decision logic used by
// the *tcell.EventResize handler: an open preview target is re-run when its currently computed
// text width differs from the width its content was last requested at, and left alone otherwise.
func TestRefreshPreviewTargetAfterResizeReRunsOnlyOnWidthChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	writeFileForPreviewView(t, path)
	h, _ := newTestHandler(t, 100, 30)

	h.mu.Lock()
	h.model.FilePreview.Open = true
	h.model.FilePreview.Phase = ui.FilePreviewPhaseDone
	h.model.FilePreview.Path = path
	h.mu.Unlock()
	tw, _, ok := h.inactivePanelPreviewLayoutMetrics(true)
	if !ok {
		t.Fatal("inactivePanelPreviewLayoutMetrics() ok = false, want true")
	}

	// Same width as last request: no re-run.
	h.previewLastWidth[previewTargetInactive] = tw
	genBefore := h.filePreviewRunGen.Load()
	h.refreshPreviewTargetAfterResize(previewTargetInactive)
	if got := h.filePreviewRunGen.Load(); got != genBefore {
		t.Fatalf("filePreviewRunGen = %d, want unchanged %d when width did not change", got, genBefore)
	}

	// Different width from last request: re-run triggered.
	h.previewLastWidth[previewTargetInactive] = tw + 1
	h.refreshPreviewTargetAfterResize(previewTargetInactive)
	if got := h.filePreviewRunGen.Load(); got != genBefore+1 {
		t.Fatalf("filePreviewRunGen = %d, want %d after width change triggers a re-run", got, genBefore+1)
	}
	if h.previewLastWidth[previewTargetInactive] != tw {
		t.Fatalf("previewLastWidth[inactive] = %d, want %d recorded from the new request", h.previewLastWidth[previewTargetInactive], tw)
	}
}
