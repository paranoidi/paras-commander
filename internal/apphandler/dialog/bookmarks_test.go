package dialog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestDefaultBookmarkName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/", want: "root"},
		{path: "/home/user/projects", want: "projects"},
		{path: ".", want: "root"},
		{path: "", want: "root"},
	}
	for _, tt := range tests {
		if got := DefaultBookmarkName(tt.path); got != tt.want {
			t.Fatalf("DefaultBookmarkName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestOpenBookmarkDialogDoesNotBlockOnDelayedFilesystem(t *testing.T) {
	h, host := newBookmarkTestHandler(t)
	release := stallBookmarkIO(t)
	harbor := filepath.Join(host.active.PathString(), "harbor")

	done := make(chan struct{})
	go func() {
		h.OpenBookmarkDialog()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("OpenBookmarkDialog blocked on delayed filesystem")
	}

	if !h.model.PathPicker.Open {
		t.Fatal("picker should open before the marks file becomes readable")
	}
	if len(h.model.PathPicker.Items) != 0 {
		t.Fatalf("items = %d, want 0 until the load completes", len(h.model.PathPicker.Items))
	}

	close(release)
	p := waitBookmarkIOPayload(t, h.screen.(tcell.SimulationScreen))
	h.ApplyBookmarkIO(p)
	if len(h.model.PathPicker.Items) != 1 || h.model.PathPicker.Items[0].Path != harbor {
		t.Fatalf("items after apply = %+v, want [%s]", h.model.PathPicker.Items, harbor)
	}
}

func TestBookmarkLoadStaleCompletionDiscarded(t *testing.T) {
	h, _ := newBookmarkTestHandler(t)
	release := stallBookmarkIO(t)

	h.OpenBookmarkDialog()
	if !h.model.PathPicker.Open {
		t.Fatal("expected picker open")
	}
	staleGen := h.pathPickerMissingGen
	h.ClosePathPicker()
	close(release)

	p := waitBookmarkIOPayload(t, h.screen.(tcell.SimulationScreen))
	if p.Gen != staleGen {
		t.Fatalf("payload gen = %d, want stalled open gen %d", p.Gen, staleGen)
	}
	h.ApplyBookmarkIO(p)
	if h.model.PathPicker.Open {
		t.Fatal("stale load must not reopen a closed picker")
	}
	if len(h.model.PathPicker.Items) != 0 {
		t.Fatalf("stale load applied %d items, want discarded", len(h.model.PathPicker.Items))
	}
}

func TestExecuteAddBookmarkDoesNotBlockOnDelayedFilesystem(t *testing.T) {
	h, host := newBookmarkTestHandler(t)
	h.OpenAddBookmarkDialog()
	if !h.model.FileDialog.Open {
		t.Fatal("expected add-bookmark dialog")
	}
	release := stallBookmarkIO(t)

	done := make(chan struct{})
	go func() {
		h.ExecuteAddBookmark()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ExecuteAddBookmark blocked on delayed filesystem")
	}
	if h.model.FileDialog.Open {
		t.Fatal("add dialog should close before the write finishes")
	}

	close(release)
	p := waitBookmarkIOPayload(t, h.screen.(tcell.SimulationScreen))
	h.ApplyBookmarkIO(p)
	if p.Err != nil {
		t.Fatalf("append: %v", p.Err)
	}
	data, err := os.ReadFile(host.cfg.Bookmarks.File)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" {
		t.Fatal("marks file empty after add completed")
	}
}

func TestAddBookmarkStaleCompletionDiscarded(t *testing.T) {
	h, host := newBookmarkTestHandler(t)
	h.OpenAddBookmarkDialog()
	release := stallBookmarkIO(t)
	h.ExecuteAddBookmark()
	staleGen := h.remoteFileOpGen
	h.remoteFileOpGen++
	close(release)

	p := waitBookmarkIOPayload(t, h.screen.(tcell.SimulationScreen))
	if p.Gen != staleGen {
		t.Fatalf("payload gen = %d, want %d", p.Gen, staleGen)
	}
	h.ApplyBookmarkIO(p)
	if len(host.messages) != 0 {
		t.Fatalf("stale add applied message %v, want discarded", host.messages)
	}
}

func TestDeleteBookmarkDoesNotBlockOnDelayedFilesystem(t *testing.T) {
	h, host := newBookmarkTestHandler(t)
	h.OpenBookmarkDialog()
	drainBookmarkIOEvents(h.screen.(tcell.SimulationScreen))
	if len(h.model.PathPicker.Items) != 1 {
		t.Fatalf("items = %d, want 1 after inline load", len(h.model.PathPicker.Items))
	}
	release := stallBookmarkIO(t)

	done := make(chan struct{})
	go func() {
		if !h.deleteSelectedBookmark() {
			t.Error("deleteSelectedBookmark returned false")
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("deleteSelectedBookmark blocked on delayed filesystem")
	}
	if len(h.model.PathPicker.Items) != 1 {
		t.Fatal("list must stay unchanged until the write completes")
	}

	close(release)
	p := waitBookmarkIOPayload(t, h.screen.(tcell.SimulationScreen))
	h.ApplyBookmarkIO(p)
	if len(h.model.PathPicker.Items) != 0 {
		t.Fatalf("items after apply = %d, want 0", len(h.model.PathPicker.Items))
	}
	data, err := os.ReadFile(host.cfg.Bookmarks.File)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("marks file = %q, want empty", data)
	}
}

func TestDeleteBookmarkStaleCompletionDiscarded(t *testing.T) {
	h, host := newBookmarkTestHandler(t)
	h.OpenBookmarkDialog()
	drainBookmarkIOEvents(h.screen.(tcell.SimulationScreen))
	release := stallBookmarkIO(t)
	if !h.deleteSelectedBookmark() {
		t.Fatal("delete")
	}
	staleGen := h.remoteFileOpGen
	h.remoteFileOpGen++
	close(release)

	p := waitBookmarkIOPayload(t, h.screen.(tcell.SimulationScreen))
	if p.Gen != staleGen {
		t.Fatalf("payload gen = %d, want %d", p.Gen, staleGen)
	}
	h.ApplyBookmarkIO(p)
	if len(h.model.PathPicker.Items) != 1 {
		t.Fatalf("stale delete removed the row, want preserved")
	}
	if len(host.messages) != 0 {
		t.Fatalf("stale delete applied message %v, want discarded", host.messages)
	}
}

func TestApplyBookmarkIOLoadFillsOpenPicker(t *testing.T) {
	h, host := newBookmarkTestHandler(t)
	h.model.PathPicker = uidialog.PathPickerState{
		Open:    true,
		Title:   "Bookmarks",
		Purpose: uidialog.PathPickerPurposeNavigate,
	}
	h.pathPickerMissingGen = 4
	harbor := filepath.Join(host.active.PathString(), "harbor")
	h.ApplyBookmarkIO(BookmarkIOPayload{
		Gen:   4,
		kind:  bookmarkIOLoad,
		Items: []uidialog.PathPickerItem{{Name: "harbor", Path: harbor}},
	})
	if len(h.model.PathPicker.Items) != 1 || h.model.PathPicker.Items[0].Name != "harbor" {
		t.Fatalf("items = %+v", h.model.PathPicker.Items)
	}
	if len(h.model.PathPicker.Ranked) != 1 {
		t.Fatalf("ranked = %v, want one entry after apply", h.model.PathPicker.Ranked)
	}
}

type bookmarkTestHost struct {
	fakeMassRenamePatternHost
	active *panel.State
}

func (f *bookmarkTestHost) ActivePanel() *panel.State { return f.active }

func newBookmarkTestHandler(t *testing.T) (*Handler, *bookmarkTestHost) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
	harbor := filepath.Join(root, "harbor")
	if err := os.Mkdir(harbor, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte("harbor : "+harbor+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(120, 40)

	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	host := &bookmarkTestHost{
		fakeMassRenamePatternHost: fakeMassRenamePatternHost{cfg: cfg},
		active:                    &panel.State{Path: pathloc.FileMust(root)},
	}
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{
		host:               host,
		screen:             screen,
		model:              &ui.Model{UserHomeDir: root},
		keysBookmarkDialog: bundle.BookmarkDialog,
	}
	return h, host
}

// stallBookmarkIO holds the next marks-file read/write until the returned channel is closed.
// Completions are left on the event queue (no inline apply) so the caller can assert
// Open/Execute/delete returned before I/O finished.
func stallBookmarkIO(t *testing.T) chan struct{} {
	t.Helper()
	release := make(chan struct{})
	bookmarkIOSkipInline.Store(true)
	bookmarkIOStall = func() { <-release }
	t.Cleanup(func() {
		bookmarkIOStall = nil
		bookmarkIOSkipInline.Store(false)
	})
	return release
}

func drainBookmarkIOEvents(screen tcell.SimulationScreen) {
	for screen.HasPendingEvent() {
		_ = screen.PollEvent()
	}
}

func waitBookmarkIOPayload(t *testing.T, screen tcell.SimulationScreen) BookmarkIOPayload {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !screen.HasPendingEvent() {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		ev := screen.PollEvent()
		ie, ok := ev.(*tcell.EventInterrupt)
		if !ok {
			continue
		}
		p, ok := ie.Data().(BookmarkIOPayload)
		if !ok {
			continue
		}
		return p
	}
	t.Fatal("timeout waiting for BookmarkIOPayload")
	return BookmarkIOPayload{}
}
