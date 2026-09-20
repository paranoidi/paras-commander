package dialog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	previewctrl "github.com/paranoidi/paras-commander/internal/apphandler/preview"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/uitest"
)

func newAttrOpHarness(t *testing.T, dir string, selected []string) (*Handler, *identityTestHost, tcell.SimulationScreen) {
	t.Helper()
	model := &ui.Model{}
	if err := model.Primary.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := model.Secondary.Load(dir); err != nil {
		t.Fatalf("Load secondary: %v", err)
	}
	if len(selected) == 1 {
		if !model.Primary.SelectVisibleEntry(filepath.Base(selected[0])) {
			t.Fatalf("%s not visible", selected[0])
		}
	} else {
		model.Primary.SelectedPaths = make(map[string]bool, len(selected))
		for _, p := range selected {
			model.Primary.SelectedPaths[filepath.Clean(p)] = true
		}
	}
	cfg := config.Default()
	cfg.Preview.Prefetch = false
	fh := &identityTestHost{model: model, cfg: cfg}
	screen := uitest.Screen(t, 80, 24)
	h := New(Deps{
		Host: fh, Screen: screen, Model: model,
		Preview: previewctrl.New(previewctrl.Deps{
			Host: fh, Screen: screen, Model: model, Ctx: context.Background(),
		}),
	})
	return h, fh, screen
}

func generateAttrFiles(t *testing.T, n int) (dir string, paths []string) {
	t.Helper()
	dir = t.TempDir()
	paths = make([]string, n)
	for i := 0; i < n; i++ {
		p := filepath.Join(dir, fmt.Sprintf("willow-%03d.txt", i))
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		paths[i] = p
	}
	return dir, paths
}

func TestExecuteChmodDoesNotBlockOnLargeSelection(t *testing.T) {
	dir, paths := generateAttrFiles(t, 80)
	h, _, _ := newAttrOpHarness(t, dir, paths)
	h.model.FileDialog = uidialog.FileDialogState{
		Open:       true,
		DialogType: uidialog.FileDialogChmod,
		Fields:     []uidialog.FileDialogField{{Label: "Mode", Value: "644"}},
	}
	gate := make(chan struct{})
	attrOpTestHook = func() { <-gate }
	t.Cleanup(func() {
		attrOpTestHook = nil
		select {
		case <-gate:
		default:
			close(gate)
		}
	})
	done := make(chan struct{})
	go func() {
		h.executeChmod()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("executeChmod blocked on batch chmod")
	}
}

func TestExecuteChmodErrorRefreshesAfterPartialFailure(t *testing.T) {
	dir, paths := generateAttrFiles(t, 3)
	h, fh, screen := newAttrOpHarness(t, dir, paths)
	h.model.FileDialog = uidialog.FileDialogState{
		Open:       true,
		DialogType: uidialog.FileDialogChmod,
		Fields:     []uidialog.FileDialogField{{Label: "Mode", Value: "644"}},
	}
	attrOpTestHook = func() {
		if err := os.Remove(paths[len(paths)-1]); err != nil {
			t.Errorf("remove: %v", err)
		}
	}
	t.Cleanup(func() { attrOpTestHook = nil })
	h.executeChmod()
	h.ApplyRemoteFileOp(waitRemoteFileOp(t, screen))
	if fh.errors == nil || !strings.Contains(strings.Join(fh.errors, "\n"), "Chmod failed") {
		t.Fatalf("errors = %v, want Chmod failed", fh.errors)
	}
	st, err := os.Stat(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o644 {
		t.Fatalf("first file mode = %o, want 644 after partial batch", st.Mode().Perm())
	}
	if h.model.FileDialog.Open {
		t.Fatal("dialog should close after completion")
	}
}

func TestExecuteChmodRefreshAfterCompletion(t *testing.T) {
	dir, paths := generateAttrFiles(t, 4)
	h, fh, screen := newAttrOpHarness(t, dir, paths)
	h.model.FileDialog = uidialog.FileDialogState{
		Open:       true,
		DialogType: uidialog.FileDialogChmod,
		Fields:     []uidialog.FileDialogField{{Label: "Mode", Value: "644"}},
	}
	h.executeChmod()
	if h.model.FileDialog.Open {
		t.Fatal("dialog should close before the batch finishes")
	}
	h.ApplyRemoteFileOp(waitRemoteFileOp(t, screen))
	if !strings.Contains(strings.Join(fh.messages, "\n"), "Changed mode") {
		t.Fatalf("messages = %v, want Changed mode", fh.messages)
	}
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0o644 {
			t.Fatalf("%s mode = %o, want 644", p, st.Mode().Perm())
		}
	}
}

func TestExecuteChownDoesNotBlockOnLargeSelection(t *testing.T) {
	dir, paths := generateAttrFiles(t, 80)
	h, _, _ := newAttrOpHarness(t, dir, paths)
	h.model.FileDialog = uidialog.FileDialogState{
		Open:       true,
		DialogType: uidialog.FileDialogChown,
		Fields: []uidialog.FileDialogField{
			{Label: "User", Value: "0"},
			{Label: "Group", Value: ""},
		},
	}
	gate := make(chan struct{})
	attrOpTestHook = func() { <-gate }
	t.Cleanup(func() {
		attrOpTestHook = nil
		select {
		case <-gate:
		default:
			close(gate)
		}
	})
	done := make(chan struct{})
	go func() {
		h.executeChown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("executeChown blocked on batch chown")
	}
}
