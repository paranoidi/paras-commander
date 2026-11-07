package preview

import (
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"

	previewrun "github.com/paranoidi/paras-commander/internal/preview"
)

func refineBody(text string) previewrun.Result {
	var cells []previewpanel.AnsiCell
	for _, r := range text {
		cells = append(cells, previewpanel.AnsiCell{R: r})
	}
	return previewrun.Result{Source: ui.PreviewSourceInternalHighlighted, HighlightedCells: cells}
}

func openRefineTarget(t *testing.T, h *Handler) (previewrun.Request, uint64) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "harbor.go")
	_, gen := h.beginPreviewRun(previewTargetInactive)
	h.mu.Lock()
	h.model.FilePreview.Open = true
	h.model.FilePreview.Path = path
	h.mu.Unlock()
	req := previewrun.Request{Path: path}
	h.applyPreviewResult(req, previewTargetInactive, gen, refineBody("walnut pepper"))
	h.mu.Lock()
	h.model.FilePreview.Selection.Anchor, h.model.FilePreview.Selection.Head = 0, 5
	h.model.FilePreview.Scroll = 3
	h.model.FilePreview.Search = previewpanel.SearchState{
		Active: true, Query: "pepper", Current: 0,
		Matches: []previewpanel.SearchMatch{{Start: 7, End: 13}},
	}
	h.mu.Unlock()
	return req, gen
}

func TestRefineKeepsSelectionAndSearchWhenCellsLineUp(t *testing.T) {
	h, _ := newTestHandler(t, 100, 30)
	req, gen := openRefineTarget(t, h)
	final := refineBody("walnut pepper")
	h.refinePreviewResult(req, previewTargetInactive, gen, final)
	h.mu.RLock()
	defer h.mu.RUnlock()
	st := h.model.FilePreview
	if &st.HighlightedCells[0] != &final.HighlightedCells[0] {
		t.Fatal("final cells were not applied")
	}
	if _, _, ok := st.Selection.Range(); !ok {
		t.Fatal("selection dropped by a same-layout refine")
	}
	if st.Search.Current != 0 || len(st.Search.Matches) != 1 {
		t.Fatalf("search = %+v", st.Search)
	}
}

func TestRefineRecomputesWhenCellsMoved(t *testing.T) {
	h, _ := newTestHandler(t, 100, 30)
	req, gen := openRefineTarget(t, h)
	h.refinePreviewResult(req, previewTargetInactive, gen, refineBody("a much longer walnut pepper"))
	h.mu.RLock()
	defer h.mu.RUnlock()
	st := h.model.FilePreview
	if _, _, ok := st.Selection.Range(); ok {
		t.Fatal("selection offsets are stale after the body moved")
	}
	if m := st.Search.Matches; len(m) != 1 || m[0].Start != 21 {
		t.Fatalf("matches = %+v, want recomputed against the new body", m)
	}
}

func TestResizeReflowsChromaBodyWithoutRun(t *testing.T) {
	h, _ := newTestHandler(t, 100, 30)
	h.mu.Lock()
	h.model.FilePreview.Open = true
	h.model.FilePreview.Phase = ui.FilePreviewPhaseDone
	h.model.FilePreview.Path = filepath.Join(t.TempDir(), "harbor.go")
	h.model.FilePreview.Source = ui.PreviewSourceInternalHighlighted
	h.model.FilePreview.ChromaStyle = "monokai"
	h.mu.Unlock()
	tw, _, ok := h.inactivePanelPreviewLayoutMetrics(true)
	if !ok {
		t.Fatal("no layout")
	}
	h.previewLastWidth[previewTargetInactive] = tw + 1
	gen := h.filePreviewRunGen.Load()
	h.refreshPreviewTargetAfterResize(previewTargetInactive)
	if got := h.filePreviewRunGen.Load(); got != gen {
		t.Fatalf("run gen %d -> %d: a Chroma body must not re-run on resize", gen, got)
	}
	if h.previewLastWidth[previewTargetInactive] != tw {
		t.Fatalf("last width = %d, want %d", h.previewLastWidth[previewTargetInactive], tw)
	}
}
