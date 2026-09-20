package preview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestQuickViewAndFullscreenKeepIndependentRunGensAcrossResize(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("word ", 80)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h, fh := newTestHandler(t, 100, 30)
	fh.cfg.Preview.Mode = config.PreviewModeInternal
	h.model.Primary = panel.State{
		Path: pathloc.MustParse(root),
		Entries: []localfs.Entry{
			{Name: "notes.txt", Path: path, Type: localfs.EntryFile, Size: 1},
		},
		Cursor: 0,
	}
	h.model.ActivePanel = ui.PrimaryPanel
	h.model.ViewMode = ui.ViewBrowser
	h.model.QuickViewEnabled = true
	h.model.QuickViewPanel = ui.PrimaryPanel
	fh.inactive = ui.SecondaryPanel

	h.applyQuickViewPreviewNow()
	qvAfterOpen := h.previewRunGenFor(previewTargetInactive).Load()
	if qvAfterOpen == 0 {
		t.Fatal("quick-view run gen stayed 0 after apply")
	}

	if err := h.OpenFullscreenFilePreviewAt(path, false); err != nil {
		t.Fatalf("OpenFullscreenFilePreviewAt: %v", err)
	}
	if got := h.previewRunGenFor(previewTargetInactive).Load(); got != qvAfterOpen {
		t.Fatalf("opening F3 bumped the quick-view run gen %d → %d", qvAfterOpen, got)
	}
	fsAfterOpen := h.previewRunGenFor(previewTargetFullscreen).Load()
	if fsAfterOpen == 0 {
		t.Fatal("fullscreen run gen stayed 0 after open")
	}

	qvTW, _, qvOK := h.inactivePanelPreviewLayoutMetrics(true)
	fsTW, fsOK := h.fullscreenPreviewTextWidth()
	if !qvOK || !fsOK {
		t.Fatal("layout metrics unavailable")
	}
	h.previewLastWidth[previewTargetInactive] = qvTW + 1
	h.previewLastWidth[previewTargetFullscreen] = fsTW + 1

	h.RefreshPreviewsAfterResize()

	qvAfterResize := h.previewRunGenFor(previewTargetInactive).Load()
	fsAfterResize := h.previewRunGenFor(previewTargetFullscreen).Load()
	if qvAfterResize != qvAfterOpen+1 {
		t.Fatalf("quick-view run gen = %d, want %d after resize (F3 refresh must not steal this gen)", qvAfterResize, qvAfterOpen+1)
	}
	if fsAfterResize != fsAfterOpen+1 {
		t.Fatalf("fullscreen run gen = %d, want %d after resize", fsAfterResize, fsAfterOpen+1)
	}
	if h.previewLastWidth[previewTargetInactive] != qvTW {
		t.Fatalf("quick-view wrap width = %d, want %d after resize", h.previewLastWidth[previewTargetInactive], qvTW)
	}
	if h.previewLastWidth[previewTargetFullscreen] != fsTW {
		t.Fatalf("fullscreen wrap width = %d, want %d after resize", h.previewLastWidth[previewTargetFullscreen], fsTW)
	}

	h.CloseFilePreviewFullscreen()
	if got := h.previewRunGenFor(previewTargetInactive).Load(); got != qvAfterResize {
		t.Fatalf("closing F3 bumped the quick-view run gen %d → %d", qvAfterResize, got)
	}
	h.mu.RLock()
	qvOpen := h.model.FilePreview.Open
	qvPath := h.model.FilePreview.Path
	h.mu.RUnlock()
	if !qvOpen || qvPath != path {
		t.Fatalf("quick view after F3 close = {Open:%v Path:%q}, want still open on %q", qvOpen, qvPath, path)
	}
	if h.previewLastWidth[previewTargetInactive] != qvTW {
		t.Fatalf("quick-view wrap width = %d after F3 close, want resized %d", h.previewLastWidth[previewTargetInactive], qvTW)
	}
}
