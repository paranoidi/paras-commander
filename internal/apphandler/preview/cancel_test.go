package preview

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	previewrun "github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func waitChan(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal(msg)
	}
}

func installBlockingPreviewExec(t *testing.T, h *Handler) (started, nextStarted, canceled chan struct{}) {
	t.Helper()
	started = make(chan struct{})
	nextStarted = make(chan struct{})
	canceled = make(chan struct{})
	var starts atomic.Int32
	var onceCancel atomic.Bool
	var wg sync.WaitGroup
	orig := currentPreviewExec()
	t.Cleanup(func() {
		h.cancelPreviewRun(previewTargetInactive)
		h.cancelPreviewRun(previewTargetFullscreen)
		h.cancelPreviewRun(previewTargetCarousel)
		wg.Wait()
		setPreviewExec(orig)
	})
	setPreviewExec(func(ctx context.Context, _ previewrun.Request) previewrun.Result {
		wg.Add(1)
		defer wg.Done()
		switch starts.Add(1) {
		case 1:
			close(started)
		case 2:
			close(nextStarted)
		}
		<-ctx.Done()
		if onceCancel.CompareAndSwap(false, true) {
			close(canceled)
		}
		return previewrun.Result{ErrorMsg: "stale-cancel"}
	})
	return started, nextStarted, canceled
}

func setupQuickViewFile(t *testing.T, h *Handler, fh *fakeHost, path, root string) {
	t.Helper()
	fh.cfg.Preview.Mode = config.PreviewModeInternal
	h.model.Primary = panel.State{
		Path: pathloc.MustParse(root),
		Entries: []localfs.Entry{
			{Name: filepath.Base(path), Path: path, Type: localfs.EntryFile, Size: 1},
		},
		Cursor: 0,
	}
	h.model.ActivePanel = ui.PrimaryPanel
	h.model.ViewMode = ui.ViewBrowser
	h.model.QuickViewEnabled = true
	h.model.QuickViewPanel = ui.PrimaryPanel
	fh.inactive = ui.SecondaryPanel
}

func TestRunPreviewSupersedeCancelsPreviousAndDropsStaleCancel(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	writeFileForPreviewView(t, path)
	second := filepath.Join(root, "other.txt")
	writeFileForPreviewView(t, second)

	h, fh := newTestHandler(t, 80, 24)
	setupQuickViewFile(t, h, fh, path, root)
	started, nextStarted, canceled := installBlockingPreviewExec(t, h)

	h.applyQuickViewPreviewNow()
	waitChan(t, started, "first preview run did not start")

	h.model.Primary.Entries[0].Path = second
	h.model.Primary.Entries[0].Name = filepath.Base(second)
	h.applyQuickViewPreviewNow()
	waitChan(t, canceled, "superseding the preview did not cancel the previous run")
	waitChan(t, nextStarted, "replacement preview run did not start")

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		h.mu.RLock()
		st := h.model.FilePreview
		h.mu.RUnlock()
		if st.ErrorMsg == "stale-cancel" {
			t.Fatalf("stale canceled run wrote ErrorMsg=%q into the newer preview", st.ErrorMsg)
		}
		time.Sleep(5 * time.Millisecond)
	}
	h.mu.RLock()
	st := h.model.FilePreview
	h.mu.RUnlock()
	if !st.Open || st.Path != second {
		t.Fatalf("FilePreview = {Open:%v Path:%q}, want Open on %q", st.Open, st.Path, second)
	}
	if st.ErrorMsg == "stale-cancel" {
		t.Fatal("stale cancel wrote into the newer preview state")
	}
}

func TestCloseFilePreviewCancelsInFlightRun(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	writeFileForPreviewView(t, path)

	h, fh := newTestHandler(t, 80, 24)
	setupQuickViewFile(t, h, fh, path, root)
	started, _, canceled := installBlockingPreviewExec(t, h)

	h.applyQuickViewPreviewNow()
	waitChan(t, started, "preview run did not start")

	h.CloseFilePreview()
	waitChan(t, canceled, "CloseFilePreview did not cancel the in-flight run")

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		h.mu.RLock()
		st := h.model.FilePreview
		h.mu.RUnlock()
		if st.ErrorMsg == "stale-cancel" || st.Open {
			t.Fatalf("closed preview = {Open:%v ErrorMsg:%q}, want empty (stale cancel must not write)", st.Open, st.ErrorMsg)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestFullscreenReloadAndCloseCancelInFlightRun(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	writeFileForPreviewView(t, path)

	h, _ := newTestHandler(t, 80, 24)
	started, _, canceled := installBlockingPreviewExec(t, h)

	if err := h.OpenFullscreenFilePreviewAt(path, false); err != nil {
		t.Fatalf("OpenFullscreenFilePreviewAt: %v", err)
	}
	waitChan(t, started, "fullscreen preview run did not start")

	reloaded := make(chan struct{})
	closed := make(chan struct{})
	// Reload (F5 / style refresh) must cancel the previous target run, then close
	// must cancel the replacement. The first run is still inside installBlockingPreviewExec.
	setPreviewExec(func(ctx context.Context, _ previewrun.Request) previewrun.Result {
		close(reloaded)
		<-ctx.Done()
		close(closed)
		return previewrun.Result{ErrorMsg: "stale-cancel"}
	})

	h.refreshFullscreenFilePreview()
	waitChan(t, canceled, "fullscreen reload did not cancel the previous run")
	waitChan(t, reloaded, "fullscreen reload did not start a replacement run")

	h.mu.RLock()
	errMsg := h.model.FullscreenFilePreview.ErrorMsg
	h.mu.RUnlock()
	if errMsg == "stale-cancel" {
		t.Fatal("stale cancel wrote into the reloaded fullscreen preview")
	}

	h.CloseFilePreviewFullscreen()
	waitChan(t, closed, "CloseFilePreviewFullscreen did not cancel the in-flight run")

	h.mu.RLock()
	st := h.model.FullscreenFilePreview
	h.mu.RUnlock()
	if st.Open || st.ErrorMsg == "stale-cancel" {
		t.Fatalf("closed fullscreen = {Open:%v ErrorMsg:%q}, want empty", st.Open, st.ErrorMsg)
	}
}
