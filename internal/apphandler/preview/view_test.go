package preview

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
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

func TestDispatchQuickViewFilePreviewStaleGenSkipsPatch(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "binary.dat")
	if err := os.WriteFile(path, []byte("a\x00b"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, _ := newTestHandler(t, 80, 24)

	h.mu.Lock()
	h.model.FilePreview.Open = true
	h.model.FilePreview.Phase = ui.FilePreviewPhasePending
	h.model.FilePreview.Path = path
	h.mu.Unlock()
	staleGen := h.filePreviewRunGen.Add(1)
	h.filePreviewRunGen.Add(1)

	req := h.previewRequest(path, 80, 20, root, false, nil, previewTargetInactive)
	h.dispatchQuickViewFilePreview(path, req, staleGen)

	h.mu.RLock()
	ph := h.model.FilePreview.Phase
	h.mu.RUnlock()
	if ph != ui.FilePreviewPhasePending {
		t.Fatalf("Phase = %v, want unchanged Pending when gen is stale (CheckFilePreviewable result for a superseded dispatch must not clobber fresher state)", ph)
	}
}

func TestDispatchQuickViewFilePreviewCurrentGenAppliesPreview(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.go")
	writeFileForPreviewView(t, path)
	h, fh := newTestHandler(t, 80, 24)
	fh.cfg.Preview.Mode = config.PreviewModeInternal

	h.mu.Lock()
	h.model.FilePreview.Open = true
	h.model.FilePreview.Phase = ui.FilePreviewPhasePending
	h.model.FilePreview.Path = path
	h.mu.Unlock()
	gen := h.filePreviewRunGen.Add(1)
	req := h.previewRequest(path, 80, 20, root, false, nil, previewTargetInactive)
	h.dispatchQuickViewFilePreview(path, req, gen)

	h.mu.RLock()
	st := h.model.FilePreview
	h.mu.RUnlock()
	if !st.Open || st.Phase != ui.FilePreviewPhaseDone {
		t.Fatalf("FilePreview = {Open:%v Phase:%v}, want Open=true Phase=Done", st.Open, st.Phase)
	}
	if st.Path != path {
		t.Fatalf("FilePreview.Path = %q, want %q", st.Path, path)
	}
}

func TestDispatchQuickViewFilePreviewCurrentGenAppliesNotPreviewableMessage(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "binary.dat")
	if err := os.WriteFile(path, []byte("a\x00b"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, _ := newTestHandler(t, 80, 24)

	h.mu.Lock()
	h.model.FilePreview.Open = true
	h.model.FilePreview.Phase = ui.FilePreviewPhasePending
	h.model.FilePreview.Path = path
	h.mu.Unlock()
	gen := h.filePreviewRunGen.Add(1)
	req := h.previewRequest(path, 80, 20, root, false, nil, previewTargetInactive)
	h.dispatchQuickViewFilePreview(path, req, gen)

	h.mu.RLock()
	st := h.model.FilePreview
	h.mu.RUnlock()
	if st.Phase != ui.FilePreviewPhaseDone || st.ErrorMsg != "Quick view: not a text file" {
		t.Fatalf("FilePreview = {Phase:%v ErrorMsg:%q}, want Done with the not-a-text-file message", st.Phase, st.ErrorMsg)
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

func TestApplyQuickViewPreviewSkipsEmptyFile(t *testing.T) {
	root := t.TempDir()
	emptyPath := filepath.Join(root, "blank.txt")
	if err := os.WriteFile(emptyPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	h, fh := newTestHandler(t, 80, 24)
	h.model.Primary = panel.State{Path: pathloc.MustParse(root)}
	if err := h.model.Primary.Load(root); err != nil {
		t.Fatal(err)
	}
	emptyIdx := -1
	for i, e := range h.model.Primary.Entries {
		if e.Name == "blank.txt" {
			emptyIdx = i
			break
		}
	}
	if emptyIdx < 0 {
		t.Fatal("blank.txt not in listing")
	}
	h.model.Primary.Cursor = emptyIdx
	h.model.ActivePanel = ui.PrimaryPanel
	h.model.ViewMode = ui.ViewBrowser
	h.model.QuickViewEnabled = true
	h.model.QuickViewPanel = ui.PrimaryPanel
	fh.inactive = ui.SecondaryPanel

	genBefore := h.filePreviewRunGen.Load()
	h.applyQuickViewPreviewNow()

	h.mu.RLock()
	st := h.model.FilePreview
	h.mu.RUnlock()
	if st.Phase != ui.FilePreviewPhaseDone || st.ErrorMsg != "Quick view: empty file" {
		t.Fatalf("FilePreview = {Phase:%v ErrorMsg:%q}, want Done with empty-file message", st.Phase, st.ErrorMsg)
	}
	if got := h.filePreviewRunGen.Load(); got != genBefore+1 {
		t.Fatalf("filePreviewRunGen = %d, want %d (invalidate in-flight on empty)", got, genBefore+1)
	}
	path, _, mode := h.quickViewWantFile()
	if mode != quickViewWantEmpty || path != emptyPath {
		t.Fatalf("quickViewWantFile = (%q, %v), want (%q, empty)", path, mode, emptyPath)
	}
}

func TestOpenFilePreviewFullscreenAllowsEmptyFile(t *testing.T) {
	root := t.TempDir()
	emptyPath := filepath.Join(root, "blank.txt")
	if err := os.WriteFile(emptyPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	h, fh := newTestHandler(t, 80, 24)
	fh.cfg.Preview.Mode = config.PreviewModeInternal
	h.model.Primary = panel.State{Path: pathloc.MustParse(root)}
	if err := h.model.Primary.Load(root); err != nil {
		t.Fatal(err)
	}
	emptyIdx := -1
	for i, e := range h.model.Primary.Entries {
		if e.Name == "blank.txt" {
			emptyIdx = i
			break
		}
	}
	if emptyIdx < 0 {
		t.Fatal("blank.txt not in listing")
	}
	h.model.Primary.Cursor = emptyIdx
	h.model.ActivePanel = ui.PrimaryPanel
	h.model.ViewMode = ui.ViewBrowser

	h.OpenFilePreviewFullscreen()

	if h.model.ViewMode != ui.ViewFilePreview {
		t.Fatalf("ViewMode = %v, want ViewFilePreview for explicit F3 on empty file", h.model.ViewMode)
	}
	h.mu.RLock()
	path := h.model.FullscreenFilePreview.Path
	h.mu.RUnlock()
	if path != emptyPath {
		t.Fatalf("FullscreenFilePreview.Path = %q, want %q", path, emptyPath)
	}
}
