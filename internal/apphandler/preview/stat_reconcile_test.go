package preview

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/preview/prefetch"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func installBlockingStat(t *testing.T, hook *func(string) (os.FileInfo, error)) (restoreBlocked func()) {
	t.Helper()
	blocked := make(chan struct{})
	orig := *hook
	t.Cleanup(func() { *hook = orig })
	*hook = func(string) (os.FileInfo, error) {
		<-blocked
		return nil, os.ErrNotExist
	}
	return func() { close(blocked) }
}

func TestSchedulePrefetchFromActivePanelDoesNotStatOnCaller(t *testing.T) {
	unblock := installBlockingStat(t, &prefetch.ResolveStat)
	defer unblock()

	handler, _ := newTestHandler(t, 80, 24)
	t.Cleanup(handler.stopPrefetch)

	root := t.TempDir()
	handler.model.Primary = panel.State{
		Path: pathloc.MustParse(root),
		Entries: []localfs.Entry{
			{Name: "meadow.png", Path: filepath.Join(root, "meadow.png"), Type: localfs.EntryFile},
			{Name: "harbor.png", Path: filepath.Join(root, "harbor.png"), Type: localfs.EntryFile},
		},
		Cursor: 0,
	}
	handler.model.ActivePanel = ui.PrimaryPanel
	handler.model.QuickViewEnabled = true

	done := make(chan struct{})
	go func() {
		handler.SchedulePrefetchFromActivePanel()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("SchedulePrefetchFromActivePanel blocked on Stat; unknown metadata must resolve in the worker")
	}
}

func TestQuickViewWantFileStripDoesNotStatOnCaller(t *testing.T) {
	unblock := installBlockingStat(t, &stripStat)
	defer unblock()

	h, _ := newTestHandler(t, 80, 24)
	root := t.TempDir()
	other := t.TempDir()
	selPath := filepath.Join(other, "notes.txt")
	h.model.Primary = panel.State{
		Path:                  pathloc.MustParse(root),
		SelectedPaths:         map[string]bool{selPath: true},
		SelectionsStripOrder:  []string{selPath},
		SelectionsStripCursor: 0,
	}
	h.model.ActivePanel = ui.PrimaryPanel
	h.model.ActiveSubFocus = ui.SubFocusSelectionsStrip

	done := make(chan struct{})
	var path string
	var mode quickViewWantMode
	go func() {
		path, _, mode = h.quickViewWantFile()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("quickViewWantFile blocked on Stat for a selections-strip highlight")
	}
	if mode != quickViewWantFile || path != selPath {
		t.Fatalf("quickViewWantFile = (%q, %v), want file %q without Stat", path, mode, selPath)
	}
}

func TestQuickViewWantFileStripUsesSelectedDirPaths(t *testing.T) {
	unblock := installBlockingStat(t, &stripStat)
	defer unblock()

	h, _ := newTestHandler(t, 80, 24)
	root := t.TempDir()
	other := t.TempDir()
	selPath := filepath.Join(other, "photos")
	h.model.Primary = panel.State{
		Path:                  pathloc.MustParse(root),
		SelectedPaths:         map[string]bool{selPath: true},
		SelectedDirPaths:      map[string]bool{selPath: true},
		SelectionsStripOrder:  []string{selPath},
		SelectionsStripCursor: 0,
	}
	h.model.ActivePanel = ui.PrimaryPanel
	h.model.ActiveSubFocus = ui.SubFocusSelectionsStrip

	_, _, mode := h.quickViewWantFile()
	if mode != quickViewWantDir {
		t.Fatalf("quickViewWantFile mode = %v, want dir from SelectedDirPaths (no Stat)", mode)
	}
}

func TestQuickViewWantFileStripUsesListingSize(t *testing.T) {
	unblock := installBlockingStat(t, &stripStat)
	defer unblock()

	h, _ := newTestHandler(t, 80, 24)
	root := t.TempDir()
	other := t.TempDir()
	outside := filepath.Join(other, "away.txt")
	emptyPath := filepath.Join(root, "blank.txt")
	filePath := filepath.Join(root, "notes.txt")
	h.model.Primary = panel.State{
		Path: pathloc.MustParse(root),
		Entries: []localfs.Entry{
			{Name: "blank.txt", Path: emptyPath, Type: localfs.EntryFile, Size: 0, ModifiedAt: time.Now()},
			{Name: "notes.txt", Path: filePath, Type: localfs.EntryFile, Size: 12, ModifiedAt: time.Now()},
		},
		SelectedPaths:         map[string]bool{emptyPath: true, outside: true},
		SelectionsStripOrder:  []string{emptyPath, filePath, outside},
		SelectionsStripCursor: 0,
	}
	h.model.ActivePanel = ui.PrimaryPanel
	h.model.ActiveSubFocus = ui.SubFocusSelectionsStrip

	path, _, mode := h.quickViewWantFile()
	if mode != quickViewWantEmpty || path != emptyPath {
		t.Fatalf("quickViewWantFile = (%q, %v), want empty %q from listing size", path, mode, emptyPath)
	}

	h.model.Primary = panel.State{
		Path: pathloc.MustParse(root),
		Entries: []localfs.Entry{
			{Name: "blank.txt", Path: emptyPath, Type: localfs.EntryFile, Size: 0, ModifiedAt: time.Now()},
			{Name: "notes.txt", Path: filePath, Type: localfs.EntryFile, Size: 12, ModifiedAt: time.Now()},
		},
		SelectedPaths:         map[string]bool{filePath: true, outside: true},
		SelectionsStripOrder:  []string{filePath, outside},
		SelectionsStripCursor: 0,
	}
	path, _, mode = h.quickViewWantFile()
	if mode != quickViewWantFile || path != filePath {
		t.Fatalf("quickViewWantFile = (%q, %v), want file %q from listing size", path, mode, filePath)
	}
}
