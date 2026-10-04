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
	"github.com/paranoidi/paras-commander/internal/ops"
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

func TestConfirmFlattenQueuesJobAfterProbe(t *testing.T) {
	h, screen, jobState, _, _ := openFlattenOnGeneratedTree(t, 5)
	h.confirmFlatten()
	if !h.model.FlattenDialog.Open {
		t.Fatal("dialog closed before probe result")
	}
	p := waitRemoteFileOp(t, screen)
	h.ApplyRemoteFileOp(p)
	if h.model.FlattenDialog.Open {
		t.Fatal("dialog should close after probe apply")
	}
	all := jobState.AllJobs()
	if len(all) != 1 {
		t.Fatalf("jobs = %d, want 1", len(all))
	}
	if all[0].Type != jobs.TypeFlatten {
		t.Fatalf("job type = %v, want flatten", all[0].Type)
	}
	if len(all[0].Sources) != 5 {
		t.Fatalf("sources = %d, want 5 generated files", len(all[0].Sources))
	}
}

func TestConfirmFlattenDoesNotWalkOnCaller(t *testing.T) {
	gate := make(chan struct{})
	ops.SetCollectFlattenTestHook(func(context.Context) { <-gate })
	t.Cleanup(func() {
		ops.SetCollectFlattenTestHook(nil)
		select {
		case <-gate:
		default:
			close(gate)
		}
	})

	h, _, jobState, _, _ := openFlattenOnGeneratedTree(t, 400)
	done := make(chan struct{})
	go func() {
		h.confirmFlatten()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("confirmFlatten blocked on source collection")
	}
	if !h.model.FlattenDialog.Open {
		t.Fatal("flatten dialog closed before collection finished")
	}
	if n := len(jobState.AllJobs()); n != 0 {
		t.Fatalf("jobs queued during confirm = %d, want 0", n)
	}
}

func TestConfirmFlattenCancelDuringPlanningDropsResult(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	ops.SetCollectFlattenTestHook(func(context.Context) {
		close(started)
		<-release
	})
	t.Cleanup(func() {
		ops.SetCollectFlattenTestHook(nil)
		select {
		case <-release:
		default:
			close(release)
		}
	})

	h, screen, jobState, _, _ := openFlattenOnGeneratedTree(t, 80)
	h.confirmFlatten()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("collection did not start")
	}
	h.CloseFlattenDialog()
	close(release)
	p := waitRemoteFileOp(t, screen)
	h.ApplyRemoteFileOp(p)
	if n := len(jobState.AllJobs()); n != 0 {
		t.Fatalf("jobs after cancel = %d, want 0", n)
	}
	if h.model.FlattenDialog.Open {
		t.Fatal("flatten dialog should stay closed")
	}
}

func TestConfirmFlattenStaleDialogCloseDropsResult(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	ops.SetCollectFlattenTestHook(func(context.Context) {
		close(started)
		<-release
	})
	t.Cleanup(func() {
		ops.SetCollectFlattenTestHook(nil)
		select {
		case <-release:
		default:
			close(release)
		}
	})

	h, screen, jobState, root, dest := openFlattenOnGeneratedTree(t, 80)
	h.confirmFlatten()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("collection did not start")
	}
	h.CloseFlattenDialog()
	h.model.FlattenDialog = uidialog.FlattenDialogState{
		Open:        true,
		Destination: uidialog.FileDialogField{Value: dest},
		Recursive:   true,
		DirRoots:    []string{root},
	}
	close(release)
	p := waitRemoteFileOp(t, screen)
	h.ApplyRemoteFileOp(p)
	if n := len(jobState.AllJobs()); n != 0 {
		t.Fatalf("stale probe queued %d jobs, want 0", n)
	}
}

func TestConfirmFlattenRootNamedItem(t *testing.T) {
	cases := []struct {
		name        string
		files       []string
		removeEmpty bool
		wantMsg     string
		wantJobs    int
	}{
		{"queued with remove-empty", []string{"orchid", "willow.txt"}, true, "", 1},
		{"refused without remove-empty", []string{"orchid", "willow.txt"}, false, "An item named like its folder can't replace it unless empty dirs are removed", 0},
		{"refused on duplicate names", []string{"cedar/orchid", "maple/orchid"}, true, "Several items would replace the same folder", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			root := filepath.Join(dir, "orchid")
			for _, f := range tc.files {
				p := filepath.Join(root, f)
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			h, screen, jobState := newFlattenConfirmHarness(t, dir)
			h.model.FlattenDialog = uidialog.FlattenDialogState{
				Open:        true,
				Destination: uidialog.FileDialogField{Value: dir},
				Recursive:   true,
				RemoveEmpty: tc.removeEmpty,
				DirRoots:    []string{root},
			}
			h.confirmFlatten()
			h.ApplyRemoteFileOp(waitRemoteFileOp(t, screen))

			all := jobState.AllJobs()
			if len(all) != tc.wantJobs {
				t.Fatalf("jobs = %d, want %d", len(all), tc.wantJobs)
			}
			if tc.wantJobs == 1 && len(all[0].FlattenDeferred) != 1 {
				t.Fatalf("deferred = %v, want 1 item", all[0].FlattenDeferred)
			}
			if tc.wantMsg == "" {
				return
			}
			if !h.model.FlattenDialog.Open {
				t.Fatal("dialog should stay open after refusal")
			}
			msgs := h.host.(*identityTestHost).messages
			if len(msgs) == 0 || msgs[len(msgs)-1] != tc.wantMsg {
				t.Fatalf("messages = %q, want last %q", msgs, tc.wantMsg)
			}
		})
	}
}
