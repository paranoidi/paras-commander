package commands

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdmacro"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/pools"
	"github.com/paranoidi/paras-commander/internal/textutil"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/workpool"
)

func TestRunForEachUnifiedBatchRunsEachEntryInItsGivenWorkDir(t *testing.T) {
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
		Entries:   entries,
		AllowDirs: true,
		BuildItem: func(localfs.Entry, cmdmacro.Context) (RunForEachBuiltItem, error) {
			return RunForEachBuiltItem{Argv: []string{"pwd"}, UserLine: "pwd"}, nil
		},
	}
	h.runForEachUnifiedBatch(context.Background(), 0, spec, []string{dirA, dirB})

	for i, ent := range entries {
		got := strings.TrimSpace(h.model.CommandsList[i].Stdout)
		want := textutil.AbsPathClean(ent.Path)
		if got != want {
			t.Fatalf("entry %d: cwd = %q, want %q (err=%q)", i, got, want, h.model.CommandsList[i].ErrorMsg)
		}
	}
}

func TestRunForEachWorkDirs(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	flat := localfs.Entry{Name: "lantern", Path: filepath.Join(root, "lantern"), Type: localfs.EntryDirectory}
	nested := localfs.Entry{Name: "thistle", Path: filepath.Join(meadow, "thistle"), Type: localfs.EntryDirectory}

	flatPanel := panel.State{Path: pathloc.FileMust(root)}
	treePanel := panel.State{Path: pathloc.FileMust(root), ListLayout: panel.ListLayoutTree}

	tests := []struct {
		name   string
		spec   RunForEachBatchSpec
		active *panel.State
		want   []string
	}{
		{"per entry", RunForEachBatchSpec{PerEntryWorkDir: true}, &flatPanel, []string{flat.Path, nested.Path}},
		{"panel directory", RunForEachBatchSpec{PanelWorkDir: true}, &treePanel, []string{root, root}},
		{"per entry wins over panel directory", RunForEachBatchSpec{PerEntryWorkDir: true, PanelWorkDir: true}, &treePanel, []string{flat.Path, nested.Path}},
		{"row directory in a flat layout", RunForEachBatchSpec{}, &flatPanel, []string{root, root}},
		{"row directory in a tree layout", RunForEachBatchSpec{}, &treePanel, []string{root, meadow}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.spec.Entries = []localfs.Entry{flat, nested}
			got := runForEachWorkDirs(tc.spec, tc.active)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("work dirs = %q, want %q", got, tc.want)
			}
		})
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
		host:      &bannerHost{pnl: &panel.State{}},
	}

	hold := RunForEachBatchSpec{
		Entries:    entry,
		AllowDirs:  true,
		PoolName:   "one",
		Background: true,
		BuildItem: func(localfs.Entry, cmdmacro.Context) (RunForEachBuiltItem, error) {
			script := "while [ ! -f " + strconv.Quote(gate) + " ]; do sleep 0.05; done"
			return RunForEachBuiltItem{Argv: []string{"sh", "-c", script}, UserLine: "hold"}, nil
		},
	}
	wait := hold
	wait.BuildItem = func(localfs.Entry, cmdmacro.Context) (RunForEachBuiltItem, error) {
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
		PoolName:  "one",
		BuildItem: func(localfs.Entry, cmdmacro.Context) (RunForEachBuiltItem, error) {
			return RunForEachBuiltItem{Argv: []string{"true"}, UserLine: "true"}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.runForEachUnifiedBatch(ctx, 0, spec, sameDirs(len(spec.Entries), dir))
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

func TestRunForEachPoolRunsEntriesConcurrently(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "log")
	words := []string{"harbor", "lantern", "meadow", "copper", "willow", "ember", "falcon", "marble"}
	entries := make([]localfs.Entry, len(words))
	for i, w := range words {
		p := filepath.Join(dir, w)
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		entries[i] = localfs.Entry{Name: w, Path: p, Type: localfs.EntryFile}
	}
	script := "echo + >> " + logPath + "; sleep 0.3; echo - >> " + logPath

	h := &Handler{
		screen:    screen,
		model:     &ui.Model{CommandsList: make([]ui.CommandRunEntry, len(entries))},
		mu:        &sync.RWMutex{},
		ctx:       context.Background(),
		workPools: workpool.NewRegistry([]pools.Def{{Name: "io", MaxParallel: 2}}),
	}
	spec := RunForEachBatchSpec{
		Entries:    entries,
		Background: true,
		AllowFiles: true,
		PoolName:   "io",
		BuildItem: func(localfs.Entry, cmdmacro.Context) (RunForEachBuiltItem, error) {
			return RunForEachBuiltItem{Argv: []string{"sh", "-c", script}, UserLine: strconv.Quote(script)}, nil
		},
	}
	h.runForEachUnifiedBatch(context.Background(), 0, spec, sameDirs(len(spec.Entries), dir))

	for i, e := range h.model.CommandsList {
		if e.Phase != ui.CommandRunDone || e.ExitCode != 0 {
			t.Fatalf("entry %d: phase=%v exit=%d err=%q", i, e.Phase, e.ExitCode, e.ErrorMsg)
		}
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	cur, peak := 0, 0
	for _, l := range strings.Fields(string(data)) {
		if l == "+" {
			cur++
			peak = max(peak, cur)
		} else {
			cur--
		}
	}
	if peak != 2 {
		t.Fatalf("max concurrency = %d, want 2", peak)
	}
}

func TestStartRunForEachBatchClearsSelectionAtStart(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	dir := t.TempDir()
	pnl, err := panel.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	pnl.SelectedPaths = map[string]bool{filepath.Join(dir, "walnut"): true}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h := &Handler{screen: screen, model: &ui.Model{}, mu: &sync.RWMutex{}, ctx: ctx, host: &bannerHost{pnl: &pnl}}
	h.StartRunForEachBatch(RunForEachBatchSpec{
		Entries:    []localfs.Entry{{Name: "harbor", Path: dir, Type: localfs.EntryDirectory}},
		AllowDirs:  true,
		Background: true,
		BuildItem: func(localfs.Entry, cmdmacro.Context) (RunForEachBuiltItem, error) {
			return RunForEachBuiltItem{Argv: []string{"sleep", "5"}, UserLine: "sleep"}, nil
		},
	})
	if len(pnl.SelectedPaths) != 0 {
		t.Fatalf("selection = %v, want cleared at start", pnl.SelectedPaths)
	}
}

// sameDirs is the per-entry row directory slice StartRunForEachBatch resolves, all equal to dir.
func sameDirs(n int, dir string) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = dir
	}
	return out
}
