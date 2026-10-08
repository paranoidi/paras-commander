package dialog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	previewctrl "github.com/paranoidi/paras-commander/internal/apphandler/preview"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/uitest"
)

func newFlattenConfirmHarness(t *testing.T, dir string) (*Handler, tcell.SimulationScreen, *jobs.State) {
	t.Helper()
	model := &ui.Model{}
	if err := model.Primary.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := model.Secondary.Load(dir); err != nil {
		t.Fatalf("Load secondary: %v", err)
	}
	cfg := config.Default()
	cfg.Preview.Prefetch = false
	fh := &identityTestHost{model: model, cfg: cfg}
	screen := uitest.Screen(t, 80, 24)
	jobState := jobs.NewState()
	jobsH := jobsctrl.New(jobsctrl.Deps{
		Host: fh, Screen: screen, Model: model, State: jobState, Config: cfg,
	})
	h := New(Deps{
		Host: fh, Screen: screen, Model: model, Jobs: jobsH,
		Preview: previewctrl.New(previewctrl.Deps{
			Host: fh, Screen: screen, Model: model, Ctx: context.Background(),
		}),
	})
	return h, screen, jobState
}

func openFlattenOnGeneratedTree(t *testing.T, files int) (h *Handler, screen tcell.SimulationScreen, jobState *jobs.State, root, dest string) {
	t.Helper()
	dir := t.TempDir()
	root = filepath.Join(dir, "orchid")
	dest = filepath.Join(dir, "pebble")
	if err := os.MkdirAll(filepath.Join(root, "willow"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < files; i++ {
		name := filepath.Join(root, "willow", fmt.Sprintf("cedar-%04d.txt", i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h, screen, jobState = newFlattenConfirmHarness(t, dir)
	if !h.model.Primary.SelectVisibleEntry("orchid") {
		t.Fatal("orchid not visible")
	}
	h.model.FlattenDialog = uidialog.FlattenDialogState{
		Open:        true,
		Destination: uidialog.FileDialogField{Value: dest},
		Recursive:   true,
		DirRoots:    []string{root},
	}
	return h, screen, jobState, root, dest
}

func waitRemoteFileOp(t *testing.T, screen tcell.SimulationScreen) RemoteFileOpPayload {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		evCh := make(chan tcell.Event, 1)
		go func() { evCh <- screen.PollEvent() }()
		select {
		case ev := <-evCh:
			interrupt, ok := ev.(*tcell.EventInterrupt)
			if !ok {
				continue
			}
			p, ok := interrupt.Data().(RemoteFileOpPayload)
			if !ok {
				continue
			}
			return p
		case <-deadline:
			t.Fatal("timed out waiting for flatten probe payload")
		}
	}
}

func TestConfirmFlattenQueuesRootsWithoutWalking(t *testing.T) {
	h, _, jobState, root, _ := openFlattenOnGeneratedTree(t, 5)
	h.confirmFlatten()
	if h.model.FlattenDialog.Open {
		t.Fatal("dialog should close after confirm")
	}
	all := jobState.AllJobs()
	if len(all) != 1 {
		t.Fatalf("jobs = %d, want 1", len(all))
	}
	if all[0].Type != jobs.TypeFlatten || !all[0].FlattenRecursive {
		t.Fatalf("job = %+v, want recursive flatten", all[0])
	}
	if len(all[0].Sources) != 1 || all[0].Sources[0].String() != root {
		t.Fatalf("sources = %v, want the root only", all[0].Sources)
	}
}

func TestConfirmFlattenRejectsDestInsideRoot(t *testing.T) {
	h, _, jobState, root, _ := openFlattenOnGeneratedTree(t, 1)
	h.model.FlattenDialog.Destination = uidialog.FileDialogField{Value: filepath.Join(root, "willow")}
	h.confirmFlatten()
	if !h.model.FlattenDialog.Open {
		t.Fatal("dialog should stay open after refusal")
	}
	if n := len(jobState.AllJobs()); n != 0 {
		t.Fatalf("jobs = %d, want 0", n)
	}
}
