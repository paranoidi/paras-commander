package preview

import (
	"context"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// TestQuickViewDirRuleDeclineFallsBackToOverlay covers the new directory dispatch path end to
// end: a [[preview.commands]] rule matches the directory ("t d") but declines (non-zero exit),
// so runDirPreviewRules must post QuickViewDirRuleDeclinedPayload, and applying it must fall
// back to the built-in directory-overlay listing (quickViewFollowDirectory) rather than leaving
// the preview stuck. SyncFollowTargetPath is stubbed to report "no target" so the fallback's
// outcome is deterministic (its "select a folder" message) without needing a real overlay
// snapshot.
func TestQuickViewDirRuleDeclineFallsBackToOverlay(t *testing.T) {
	h, fh := newTestHandler(t, 80, 24)
	dirPath := t.TempDir()
	fh.cfg.Preview.Commands = []config.PreviewCommandRule{
		{When: []string{"t d"}, Command: "sh -c 'exit 1'"},
	}
	fh.syncFollowTargetPath = func(*panel.State) (string, bool) { return "", false }

	h.mu.Lock()
	h.model.FilePreview.Open = true
	h.model.FilePreview.Phase = ui.FilePreviewPhasePending
	h.model.FilePreview.Path = dirPath
	h.model.FilePreview.IsDir = true
	h.mu.Unlock()

	gen := h.filePreviewRunGen.Add(1)
	req := h.previewRequest(dirPath, 80, 20, dirPath, false, nil, previewTargetInactive, true)
	h.runDirPreviewRules(context.Background(), req, gen)

	ev := h.screen.PollEvent()
	interrupt, ok := ev.(*tcell.EventInterrupt)
	if !ok {
		t.Fatalf("event = %T, want *tcell.EventInterrupt", ev)
	}
	payload, ok := interrupt.Data().(QuickViewDirRuleDeclinedPayload)
	if !ok {
		t.Fatalf("interrupt data = %T, want QuickViewDirRuleDeclinedPayload", interrupt.Data())
	}

	if !h.ApplyQuickViewDirRuleDeclined(payload) {
		t.Fatal("ApplyQuickViewDirRuleDeclined = false, want true (repaint needed)")
	}
	h.mu.RLock()
	errMsg := h.model.FilePreview.ErrorMsg
	h.mu.RUnlock()
	if errMsg != "Quick view: select a folder" {
		t.Fatalf("FilePreview.ErrorMsg = %q, want the overlay fallback's \"select a folder\" message (SyncFollowTargetPath stubbed to fail)", errMsg)
	}
}

// TestQuickViewDirRuleMatchAppliesPreviewResult covers the success side of directory dispatch: a
// matching, zero-exit rule's output becomes the quick-view preview body via the normal
// applyPreviewResult path, instead of the directory-overlay listing.
func TestQuickViewDirRuleMatchAppliesPreviewResult(t *testing.T) {
	h, fh := newTestHandler(t, 80, 24)
	dirPath := t.TempDir()
	fh.cfg.Preview.Commands = []config.PreviewCommandRule{
		{When: []string{"t d"}, Command: "sh -c 'echo tree-output; exit 0'"},
	}

	h.mu.Lock()
	h.model.FilePreview.Open = true
	h.model.FilePreview.Phase = ui.FilePreviewPhasePending
	h.model.FilePreview.Path = dirPath
	h.model.FilePreview.IsDir = true
	h.mu.Unlock()

	gen := h.filePreviewRunGen.Add(1)
	req := h.previewRequest(dirPath, 80, 20, dirPath, false, nil, previewTargetInactive, true)
	h.runDirPreviewRules(context.Background(), req, gen)

	h.mu.RLock()
	st := h.model.FilePreview
	h.mu.RUnlock()
	if st.Phase != ui.FilePreviewPhaseDone {
		t.Fatalf("Phase = %v, want Done", st.Phase)
	}
	if st.CombinedText != "tree-output\n" {
		t.Fatalf("CombinedText = %q, want %q", st.CombinedText, "tree-output\n")
	}
}

// TestApplyQuickViewDirRuleDeclinedStaleGenIgnored mirrors the gen-guard pattern used elsewhere
// in this package (e.g. TestFilePreviewRunGenStaleSkipsRunningPatch): a decline payload from a
// superseded run must not clobber state for whatever the user has already moved on to.
func TestApplyQuickViewDirRuleDeclinedStaleGenIgnored(t *testing.T) {
	h, _ := newTestHandler(t, 80, 24)
	dirPath := t.TempDir()

	h.mu.Lock()
	h.model.FilePreview.Open = true
	h.model.FilePreview.Path = dirPath
	h.model.FilePreview.IsDir = true
	h.mu.Unlock()

	staleGen := h.filePreviewRunGen.Add(1)
	h.filePreviewRunGen.Add(1) // supersede: the user moved on before the decline arrived

	if h.ApplyQuickViewDirRuleDeclined(QuickViewDirRuleDeclinedPayload{gen: staleGen}) {
		t.Fatal("ApplyQuickViewDirRuleDeclined = true, want false for a superseded gen")
	}
	h.mu.RLock()
	open := h.model.FilePreview.Open
	h.mu.RUnlock()
	if !open {
		t.Fatal("FilePreview.Open = false, want unchanged: a stale decline must not close the preview")
	}
}
