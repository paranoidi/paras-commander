package commands

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pools"
	"github.com/paranoidi/paras-commander/internal/textutil"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/workpool"
)

func TestRunForEachUnifiedBatchPerEntryWorkDir(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()

	dirA := t.TempDir()
	dirB := t.TempDir()
	entries := []localfs.Entry{
		{Name: "dirA", Path: dirA, Type: localfs.EntryDirectory},
		{Name: "dirB", Path: dirB, Type: localfs.EntryDirectory},
	}

	h := &Handler{
		screen: screen,
		model:  &ui.Model{CommandsList: make([]ui.CommandRunEntry, len(entries))},
		mu:     &sync.RWMutex{},
		ctx:    context.Background(),
	}

	spec := RunForEachBatchSpec{
		Entries:         entries,
		AllowDirs:       true,
		WorkDir:         "/should-not-be-used",
		PerEntryWorkDir: true,
		BuildItem: func(localfs.Entry) (RunForEachBuiltItem, error) {
			return RunForEachBuiltItem{Argv: []string{"pwd"}, UserLine: "pwd"}, nil
		},
	}
	h.runForEachUnifiedBatch(context.Background(), 0, spec)

	for i, ent := range entries {
		got := strings.TrimSpace(h.model.CommandsList[i].Stdout)
		want := textutil.AbsPathClean(ent.Path)
		if got != want {
			t.Fatalf("entry %d: cwd = %q, want %q (err=%q)", i, got, want, h.model.CommandsList[i].ErrorMsg)
		}
	}
}

func TestSummarizeRunForEachIssuesFailed(t *testing.T) {
	entries := []ui.CommandRunEntry{
		{ExitCode: 1, ErrorMsg: "boom"},
		{ExitCode: 0},
	}
	log, banner, urg, ok := summarizeRunForEachIssues("Run for each", entries)
	if !ok {
		t.Fatal("expected issue summary")
	}
	if urg != ui.MessageUrgencyError {
		t.Fatalf("urgency = %v", urg)
	}
	if log != "Run for each: 1 failed (boom)" {
		t.Fatalf("log = %q", log)
	}
	if banner == "" {
		t.Fatal("expected banner")
	}
}

func TestSummarizeRunForEachIssuesOK(t *testing.T) {
	_, _, _, ok := summarizeRunForEachIssues("Run for each", []ui.CommandRunEntry{{ExitCode: 0}})
	if ok {
		t.Fatal("expected no summary for all-success batch")
	}
}

func TestRunForEachPoolWaitingStaysPending(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()

	dir := t.TempDir()
	gate := filepath.Join(dir, "release")
	entry := []localfs.Entry{{Name: "harbor", Path: dir, Type: localfs.EntryDirectory}}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h := &Handler{
		screen:    screen,
		model:     &ui.Model{},
		mu:        &sync.RWMutex{},
		ctx:       ctx,
		workPools: workpool.NewRegistry([]pools.Def{{Name: "one", MaxParallel: 1}}),
	}

	hold := RunForEachBatchSpec{
		Entries:    entry,
		AllowDirs:  true,
		WorkDir:    dir,
		PoolName:   "one",
		Background: true,
		BuildItem: func(localfs.Entry) (RunForEachBuiltItem, error) {
			script := "while [ ! -f " + strconv.Quote(gate) + " ]; do sleep 0.05; done"
			return RunForEachBuiltItem{Argv: []string{"sh", "-c", script}, UserLine: "hold"}, nil
		},
	}
	wait := hold
	wait.BuildItem = func(localfs.Entry) (RunForEachBuiltItem, error) {
		return RunForEachBuiltItem{Argv: []string{"true"}, UserLine: "true"}, nil
	}

	h.StartRunForEachBatch(hold)
	waitCommandPhase(t, h, 0, ui.CommandRunRunning)
	h.StartRunForEachBatch(wait)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.RLock()
		running, pending := 0, 0
		for _, e := range h.model.CommandsList {
			switch e.Phase {
			case ui.CommandRunRunning:
				running++
			case ui.CommandRunPending:
				pending++
			}
		}
		h.mu.RUnlock()
		if running == 1 && pending == 1 {
			if err := os.WriteFile(gate, []byte("1"), 0o644); err != nil {
				t.Fatal(err)
			}
			cancel()
			waitBatchesIdle(t, h)
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	h.mu.RLock()
	dump := append([]ui.CommandRunEntry(nil), h.model.CommandsList...)
	h.mu.RUnlock()
	t.Fatalf("expected one Running and one Pending while the one-slot pool is held; rows=%v", dumpPhases(dump))
}

func waitCommandPhase(t *testing.T, h *Handler, idx int, want ui.CommandRunPhase) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.RLock()
		ok := idx < len(h.model.CommandsList) && h.model.CommandsList[idx].Phase == want
		h.mu.RUnlock()
		if ok {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	h.mu.RLock()
	dump := append([]ui.CommandRunEntry(nil), h.model.CommandsList...)
	h.mu.RUnlock()
	t.Fatalf("timed out waiting for row %d phase %v; rows=%v", idx, want, dumpPhases(dump))
}

func dumpPhases(entries []ui.CommandRunEntry) string {
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(e.UserCommandLine)
		b.WriteString(" phase=")
		switch e.Phase {
		case ui.CommandRunPending:
			b.WriteString("pending")
		case ui.CommandRunRunning:
			b.WriteString("running")
		case ui.CommandRunDone:
			b.WriteString("done")
		default:
			b.WriteString("?")
		}
		if e.ErrorMsg != "" {
			b.WriteString(" err=")
			b.WriteString(e.ErrorMsg)
		}
	}
	return b.String()
}

func waitBatchesIdle(t *testing.T, h *Handler) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !h.HasRunning() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for command batches to finish")
}

func TestRunForEachCancelWhilePoolWaiting(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()

	dir := t.TempDir()
	reg := workpool.NewRegistry([]pools.Def{{Name: "one", MaxParallel: 1}})
	release, err := reg.Acquire(context.Background(), "one")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer release()

	h := &Handler{
		screen:    screen,
		model:     &ui.Model{CommandsList: []ui.CommandRunEntry{{Phase: ui.CommandRunPending, ExitCode: -1}}},
		mu:        &sync.RWMutex{},
		ctx:       context.Background(),
		workPools: reg,
	}
	spec := RunForEachBatchSpec{
		Entries:   []localfs.Entry{{Name: "lantern", Path: dir, Type: localfs.EntryDirectory}},
		AllowDirs: true,
		WorkDir:   dir,
		PoolName:  "one",
		BuildItem: func(localfs.Entry) (RunForEachBuiltItem, error) {
			return RunForEachBuiltItem{Argv: []string{"true"}, UserLine: "true"}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.runForEachUnifiedBatch(ctx, 0, spec)
	}()

	deadline := time.Now().Add(2 * time.Second)
	seenPending := false
	for time.Now().Before(deadline) {
		h.mu.RLock()
		e := h.model.CommandsList[0]
		h.mu.RUnlock()
		if e.Phase == ui.CommandRunRunning {
			t.Fatal("pool-waiting row flipped to Running before Acquire returned")
		}
		if e.Phase == ui.CommandRunPending && e.UserCommandLine == "true" {
			seenPending = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !seenPending {
		t.Fatal("expected Pending while waiting for a pool slot")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("batch did not return after cancel while waiting for a pool slot")
	}

	h.mu.RLock()
	e := h.model.CommandsList[0]
	h.mu.RUnlock()
	if e.Phase != ui.CommandRunDone {
		t.Fatalf("phase = %v, want Done", e.Phase)
	}
	if e.ErrorMsg != "Canceled" {
		t.Fatalf("ErrorMsg = %q, want Canceled", e.ErrorMsg)
	}
}
