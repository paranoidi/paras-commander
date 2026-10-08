package meta

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// fakeHost is a minimal Host stub for handler-logic tests that don't need a real *App.
type fakeHost struct {
	editedPath  string
	editErr     error
	messages    []string
	panels      [2]*panel.State
	resortCalls []int
	// resolvedCalls records NoteMetaColumnResolved panelIDs, in call order.
	resolvedCalls []int
}

func (f *fakeHost) SetTransientMessage(text string, _ ui.MessageUrgency) {
	f.messages = append(f.messages, text)
}
func (f *fakeHost) SetErrorMessage(_ string, _ error) {}
func (f *fakeHost) PanelByID(id int) *panel.State {
	if id < 0 || id > 1 {
		return nil
	}
	return f.panels[id]
}
func (f *fakeHost) ResortPanel(id int) {
	f.resortCalls = append(f.resortCalls, id)
	if p := f.PanelByID(id); p != nil {
		p.ApplySortFromDialog(p.Sort, 1000)
	}
}
func (f *fakeHost) NoteMetaColumnResolved(id int) {
	f.resolvedCalls = append(f.resolvedCalls, id)
}
func (f *fakeHost) IconMetaRunning() string { return "*" }
func (f *fakeHost) OpenFileInExternalEditor(path string) error {
	f.editedPath = path
	return f.editErr
}
func (f *fakeHost) MessageLogWrapCols() int { return 80 }
func (f *fakeHost) AppendTransientMessageLines(banner string, _ []string, _ ui.MessageUrgency) {
	f.messages = append(f.messages, banner)
}
func (f *fakeHost) ClearTransientMessage() {}

func TestRunCommand_expandsF(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/my file.txt"
	out, err := runCommand(context.Background(), "echo %f", path, dir)
	if err != nil {
		t.Fatalf("runCommand: %v", err)
	}
	if out != path {
		t.Fatalf("out = %q, want %q", out, path)
	}
}

func TestRunCommand_hostilePathMacros(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cases := []struct {
		name string
		base string
	}{
		{"command substitution", "beacon$(echo INJECTED)"},
		{"backticks", "lantern`echo INJECTED`"},
		{"dollar home", "meadow$HOME"},
		{"double quotes", `harbor "quoted"`},
		{"single quote", "harbor's-lantern"},
		{"spaces", "orchard meadow.txt"},
		{"embedded newline", "meadow\norchard"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := dir + "/" + tc.base
			out, err := runCommand(context.Background(), "printf '%s\\n' %f", path, dir)
			if err != nil {
				t.Fatalf("runCommand: %v", err)
			}
			if out != path {
				t.Fatalf("out = %q, want %q", out, path)
			}
		})
	}
}

func TestRunCommand_success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out, err := runCommand(context.Background(), "echo hello", dir+"/file", dir)
	if err != nil {
		t.Fatalf("runCommand: %v", err)
	}
	if out != "hello" {
		t.Fatalf("out = %q, want hello", out)
	}
}

func TestRunCommand_failure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := runCommand(context.Background(), "exit 1", dir+"/file", dir)
	if err == nil {
		t.Fatal("expected error for failing command")
	}
}

func TestRunCommand_cancelled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runCommand(ctx, "echo hello", dir+"/file", dir)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestRunCommand_oversizedStdout(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("head /dev/zero not available on Windows")
	}
	dir := t.TempDir()
	out, err := runCommand(context.Background(), "head -c 600000 /dev/zero", dir+"/file", dir)
	if err != nil {
		t.Fatalf("runCommand: %v", err)
	}
	if len(out) > cmdrun.MaxStreamBytes {
		t.Fatalf("stdout %d bytes, want <= %d (bounded capture)", len(out), cmdrun.MaxStreamBytes)
	}
}

func TestRunCommand_parentExitsWhileChildLives(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("process-group capture is Unix-only")
	}
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	t.Cleanup(func() { killPIDFile(pidFile) })

	script := fmt.Sprintf(`set +m; (trap "" HUP; exec sleep 120) & echo $! > %q; echo done`, pidFile)
	start := time.Now()
	out, err := runCommandOrTimeout(t, context.Background(), script, dir+"/file", dir)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("runCommand blocked %v waiting on a descendant pipe", elapsed)
	}
	if err != nil {
		t.Fatalf("runCommand: %v", err)
	}
	if out != "done" {
		t.Fatalf("out = %q, want done", out)
	}
}

func TestRunCommand_cancelKillsProcessGroup(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("process-group cancel is Unix-only")
	}
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	t.Cleanup(func() { killPIDFile(pidFile) })

	script := fmt.Sprintf(`set +m; (trap "" HUP; exec sleep 120) & echo $! > %q; exec sleep 120`, pidFile)
	ctx, cancel := context.WithCancel(context.Background())
	type result struct {
		out string
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := runCommand(ctx, script, dir+"/file", dir)
		done <- result{out, err}
	}()

	childPID := waitPIDFile(t, pidFile, 5*time.Second)
	cancel()

	select {
	case res := <-done:
		if res.err == nil {
			t.Fatal("cancelled run succeeded; want the deadline to stop the process group")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runCommand did not return after cancel")
	}

	deadline := time.Now().Add(2 * time.Second)
	for processAlive(childPID) {
		if time.Now().After(deadline) {
			t.Fatalf("descendant pid %d still alive after cancel", childPID)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func runCommandOrTimeout(t *testing.T, ctx context.Context, cmd, path, dir string) (string, error) {
	t.Helper()
	type result struct {
		out string
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := runCommand(ctx, cmd, path, dir)
		done <- result{out, err}
	}()
	select {
	case res := <-done:
		return res.out, res.err
	case <-time.After(3 * time.Second):
		t.Fatal("runCommand did not return within 3s; a descendant is likely pinning the output pipes")
	}
	return "", nil
}

func waitPIDFile(t *testing.T, path string, timeout time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(path)
		if err == nil {
			pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
			if err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("pid file %s not written", path)
	return 0
}

func killPIDFile(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func TestScheduleRenderDebounced_burstWakesCoalesceWithoutRace(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)

	h := &Handler{screen: screen, model: &ui.Model{}, host: &fakeHost{panels: [2]*panel.State{{}, {}}}}
	h.model.MetaResults[0] = []ui.MetaColumnState{
		{EntryName: "size", Results: map[string]string{"/p": ""}},
	}
	h.runGen[0] = 1

	wake := func() {
		h.HandleWake(WakePayload{
			PanelID:   0,
			EntryName: "size",
			Path:      "/p",
			Value:     "ok",
			Gen:       1,
		})
	}

	for range 40 {
		wake()
	}

	// Straddle the ~16ms timer expiry so the callback races HandleWake's
	// timer-field access if the callback still writes Handler state.
	deadline := time.Now().Add(40 * time.Millisecond)
	for time.Now().Before(deadline) {
		wake()
		time.Sleep(time.Millisecond)
	}
	time.Sleep(25 * time.Millisecond)

	flushes := 0
	for screen.HasPendingEvent() {
		ev := screen.PollEvent()
		ie, ok := ev.(*tcell.EventInterrupt)
		if !ok {
			continue
		}
		if _, ok := ie.Data().(RenderFlushPayload); ok {
			flushes++
			h.HandleRenderFlush()
		}
	}
	if flushes < 1 {
		t.Fatal("expected at least one coalesced RenderFlushPayload")
	}
	if flushes > 8 {
		t.Fatalf("flushes = %d, want ~60fps coalescing, not one per wake", flushes)
	}
}

func TestLoadMetaFile_duplicateNamesAreError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "meta.toml"), []byte(`
[[entry]]
name = "size"
description = "Disk size"
file = "echo a"

[[entry]]
name = "size"
description = "Line count"
file = "echo b"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	fh := &fakeHost{panels: [2]*panel.State{testPanel(t, dir, nil)}}
	h := &Handler{host: fh, model: &ui.Model{}, config: config.Default()}
	if mf := h.loadMetaFile(0); mf != nil {
		t.Fatal("expected load to fail on duplicate names")
	}
	if len(fh.messages) == 0 {
		t.Fatal("expected a transient error for duplicate names")
	}
}

func TestApplyWakeResult_updatesCorrectColumn(t *testing.T) {
	h := &Handler{model: &ui.Model{}}
	h.model.MetaResults[0] = []ui.MetaColumnState{
		{EntryName: "a", Results: map[string]string{"/p": ""}},
		{EntryName: "b", Results: map[string]string{"/p": ""}},
	}
	h.applyWakeResult(WakePayload{PanelID: 0, EntryName: "b", Path: "/p", Value: "ok"})
	if got := h.model.MetaResults[0][1].Results["/p"]; got != "ok" {
		t.Fatalf("column b = %q, want ok", got)
	}
	if got := h.model.MetaResults[0][0].Results["/p"]; got != "" {
		t.Fatalf("column a = %q, want empty", got)
	}
}

// TestHandleRenderFlush_notifiesHostOnlyWhenColumnResolved confirms the coalesced render flush
// never sorts directly (there is no Sort call to assert against, since the fake host's ResortPanel
// is only used for the meta-columns-cleared path) and signals NoteMetaColumnResolved exactly once
// per panel, only once every dispatched cell has stopped reading the Pending marker.
func TestHandleRenderFlush_notifiesHostOnlyWhenColumnResolved(t *testing.T) {
	fh := &fakeHost{panels: [2]*panel.State{
		{Sort: panel.SortState{Mode: panel.SortMeta, MetaColumn: "info"}},
		{},
	}}
	h := &Handler{host: fh, model: &ui.Model{}}
	h.model.MetaResults[0] = []ui.MetaColumnState{
		{EntryName: "info", Pending: "*", PendingCount: 1, Results: map[string]string{"/a": "*", "/b": "1"}},
	}
	h.runGen[0] = 1

	h.HandleWake(WakePayload{PanelID: 0, EntryName: "info", Path: "/b", Value: "1", Gen: 1})
	h.HandleRenderFlush()
	if len(fh.resolvedCalls) != 0 {
		t.Fatalf("resolvedCalls = %v, want none while /a is still pending", fh.resolvedCalls)
	}

	h.HandleWake(WakePayload{PanelID: 0, EntryName: "info", Path: "/a", Value: "2", Gen: 1})
	h.HandleRenderFlush()
	if len(fh.resolvedCalls) != 1 || fh.resolvedCalls[0] != 0 {
		t.Fatalf("resolvedCalls = %v, want [0] once the column fully resolved", fh.resolvedCalls)
	}
}

func TestOpenFileEditor_clearsCache(t *testing.T) {
	dir := t.TempDir()
	metaPath := dir + "/meta.toml"
	if err := os.WriteFile(metaPath, []byte("[[entry]]\nname = \"lines\"\ndescription = \"Line count\"\nfile = \"wc -l\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fh := &fakeHost{}
	h := &Handler{host: fh, model: &ui.Model{}}
	h.cache = map[string]map[string]string{"lines": {"/some/file": "42"}}

	if !h.OpenFileEditor(metaPath) {
		t.Fatal("OpenFileEditor should succeed")
	}
	if fh.editedPath != metaPath {
		t.Fatalf("edited path = %q, want %q", fh.editedPath, metaPath)
	}

	h.cacheMu.RLock()
	empty := len(h.cache) == 0
	h.cacheMu.RUnlock()
	if !empty {
		t.Fatalf("cache = %#v, want cleared", h.cache)
	}
}

func testPanel(t *testing.T, dir string, entries []localfs.Entry) *panel.State {
	t.Helper()
	p, err := pathloc.Parse(dir)
	if err != nil {
		t.Fatal(err)
	}
	return &panel.State{Path: p, Entries: entries}
}

func unblockFIFO(path string) {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return
	}
	_ = f.Close()
}

func TestReconcileForPanel_doesNotReadMetaOnEventLoop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("named pipe blocking load is Unix-only")
	}
	dir := t.TempDir()
	fifo := filepath.Join(dir, "meta.toml")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { unblockFIFO(fifo) })

	alpha := filepath.Join(dir, "alpha")
	bravo := filepath.Join(dir, "bravo")
	fh := &fakeHost{panels: [2]*panel.State{testPanel(t, dir, []localfs.Entry{
		{Name: "alpha", Path: alpha, Type: localfs.EntryFile},
		{Name: "bravo", Path: bravo, Type: localfs.EntryFile},
	})}}
	h := &Handler{host: fh, model: &ui.Model{}, config: config.Default()}
	h.activeEntries[0] = []string{"size"}
	h.model.MetaResults[0] = []ui.MetaColumnState{{
		EntryName: "size",
		Results:   map[string]string{alpha: "1"},
	}}

	done := make(chan struct{})
	go func() {
		h.ReconcileForPanel(0)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("ReconcileForPanel blocked on meta.toml read")
	}
}

func TestActivateSelection_doesNotReadMetaOnEventLoop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("named pipe blocking load is Unix-only")
	}
	dir := t.TempDir()
	fifo := filepath.Join(dir, "meta.toml")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { unblockFIFO(fifo) })

	fh := &fakeHost{panels: [2]*panel.State{testPanel(t, dir, nil)}}
	h := &Handler{host: fh, model: &ui.Model{}, config: config.Default()}
	h.model.MetaDialog = dialog.MetaDialogState{
		Open:    true,
		PanelID: 0,
		Entries: []dialog.MetaEntry{{Name: "size", Description: "Size"}},
		Checked: []bool{true},
	}

	done := make(chan struct{})
	go func() {
		h.ActivateSelection()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("ActivateSelection blocked on meta.toml read")
	}
	if got := h.activeEntries[0]; len(got) != 1 || got[0] != "size" {
		t.Fatalf("activeEntries = %v, want [size] before the load finishes", got)
	}
}

func TestHandleLoad_rejectsStaleSelection(t *testing.T) {
	dir := t.TempDir()
	fh := &fakeHost{panels: [2]*panel.State{testPanel(t, dir, nil)}}
	h := &Handler{host: fh, model: &ui.Model{}, config: config.Default()}
	h.loadGen[0] = 1
	h.activeEntries[0] = []string{"new-col"}
	h.navPath[0] = filepath.Clean(dir)
	h.model.MetaResults[0] = []ui.MetaColumnState{{EntryName: "new-col"}}

	h.HandleLoad(LoadPayload{
		PanelID:     0,
		LoadGen:     1,
		Path:        filepath.Clean(dir),
		ActiveNames: []string{"old-col"},
		MF: &metacmds.MetaFile{Entries: []metacmds.MetaEntry{
			{Name: "old-col", Description: "Old", File: "echo old"},
		}},
	})
	if got := h.model.MetaResults[0][0].EntryName; got != "new-col" {
		t.Fatalf("stale load restored %q, want new-col", got)
	}
}

func TestHandleLoad_rejectsStalePath(t *testing.T) {
	dir := t.TempDir()
	fh := &fakeHost{panels: [2]*panel.State{testPanel(t, dir, nil)}}
	h := &Handler{host: fh, model: &ui.Model{}, config: config.Default()}
	h.loadGen[0] = 1
	h.activeEntries[0] = []string{"size"}
	h.model.MetaResults[0] = []ui.MetaColumnState{{EntryName: "size"}}

	h.HandleLoad(LoadPayload{
		PanelID:     0,
		LoadGen:     1,
		Path:        filepath.Join(dir, "gone"),
		ActiveNames: []string{"size"},
		MF: &metacmds.MetaFile{Entries: []metacmds.MetaEntry{
			{Name: "size", Description: "Size", Column: "bytes", File: "echo 1"},
		}},
	})
	if got := h.model.MetaResults[0][0].ColumnTitle; got != "" {
		t.Fatalf("stale-path load applied column %q", got)
	}
}

func TestActivateSelection_invalidatesPendingLoad(t *testing.T) {
	dir := t.TempDir()
	fh := &fakeHost{panels: [2]*panel.State{testPanel(t, dir, nil)}}
	h := &Handler{host: fh, model: &ui.Model{}, config: config.Default()}
	h.loadGen[0] = 1
	h.activeEntries[0] = []string{"old-col"}
	h.model.MetaResults[0] = []ui.MetaColumnState{{EntryName: "old-col"}}
	h.model.MetaDialog = dialog.MetaDialogState{
		Open:    true,
		PanelID: 0,
		Entries: []dialog.MetaEntry{{Name: "new-col", Description: "New"}},
		Checked: []bool{true},
	}

	h.ActivateSelection()

	h.HandleLoad(LoadPayload{
		PanelID:     0,
		LoadGen:     1,
		Path:        filepath.Clean(dir),
		ActiveNames: []string{"old-col"},
		MF: &metacmds.MetaFile{Entries: []metacmds.MetaEntry{
			{Name: "old-col", Description: "Old", File: "echo old"},
		}},
	})
	if got := h.activeEntries[0]; len(got) != 1 || got[0] != "new-col" {
		t.Fatalf("activeEntries = %v, want [new-col]", got)
	}
	if got := h.model.MetaResults[0][0].EntryName; got != "new-col" {
		t.Fatalf("pending load restored %q, want new-col", got)
	}
}

func TestActivateSelection_sortOnActivation(t *testing.T) {
	dir := t.TempDir()
	newDialog := func() dialog.MetaDialogState {
		return dialog.MetaDialogState{
			Open:    true,
			PanelID: 0,
			Entries: []dialog.MetaEntry{
				{Name: "score", Description: "Score", SortOnActivation: true, SortReverse: true},
			},
			Checked: []bool{true},
		}
	}

	fh := &fakeHost{panels: [2]*panel.State{testPanel(t, dir, nil)}}
	h := &Handler{host: fh, model: &ui.Model{}, config: config.Default()}
	h.model.MetaDialog = newDialog()

	h.ActivateSelection()

	p := fh.panels[0]
	if p.Sort.Mode != panel.SortMeta || p.Sort.MetaColumn != "score" || !p.Sort.Reverse {
		t.Fatalf("Sort = %+v, want Mode=SortMeta MetaColumn=score Reverse=true", p.Sort)
	}
	if len(fh.resortCalls) != 1 || fh.resortCalls[0] != 0 {
		t.Fatalf("resortCalls = %v, want [0]", fh.resortCalls)
	}

	// Re-activating with the column already active must not clobber a sort the user
	// changed manually in the meantime.
	p.Sort = panel.SortState{Mode: panel.SortName}
	h.model.MetaDialog = newDialog()

	h.ActivateSelection()

	if p.Sort.Mode != panel.SortName {
		t.Fatalf("Sort.Mode = %v after re-activation, want SortName (manual choice preserved)", p.Sort.Mode)
	}
}

// TestRunForPanel_fullyCachedRunSignalsResolvedWithoutWake covers a run that dispatches nothing
// for the sorted column (every entry already cached): runForPanel must signal
// NoteMetaColumnResolved itself, since no command runs and so no WakePayload ever arrives to
// trigger HandleRenderFlush's own check.
func TestRunForPanel_fullyCachedRunSignalsResolvedWithoutWake(t *testing.T) {
	dir := t.TempDir()
	orchard := filepath.Join(dir, "orchard.txt")
	lantern := filepath.Join(dir, "lantern.txt")
	entries := []localfs.Entry{
		{Name: "orchard.txt", Path: orchard, Type: localfs.EntryFile},
		{Name: "lantern.txt", Path: lantern, Type: localfs.EntryFile},
	}
	p := testPanel(t, dir, entries)
	p.Sort = panel.SortState{Mode: panel.SortMeta, MetaColumn: "wordcount"}

	fh := &fakeHost{panels: [2]*panel.State{p}}
	h := &Handler{host: fh, model: &ui.Model{}, config: config.Default()}
	h.cache = map[string]map[string]string{
		"wordcount": {orchard: "3", lantern: "5"},
	}

	cmdDef := metacmds.MetaEntry{Name: "wordcount", File: "wc -w %f", Cache: true}
	cols := []ui.MetaColumnState{{EntryName: "wordcount", ColumnTitle: "wordcount"}}
	h.runForPanel(0, []metacmds.MetaEntry{cmdDef}, cols)

	if len(fh.resolvedCalls) != 1 || fh.resolvedCalls[0] != 0 {
		t.Fatalf("resolvedCalls = %v, want [0] since every entry was already cached", fh.resolvedCalls)
	}
	if got := h.model.MetaResults[0][0].PendingCount; got != 0 {
		t.Fatalf("PendingCount = %d, want 0 (nothing dispatched)", got)
	}
}

// TestHandlePanelDirChanged_cancelsStaleRunAndRejectsOldWakes covers a directory change while a
// meta run is in flight: HandlePanelDirChanged must cancel the old run and bump its generation so
// a WakePayload carrying the old generation is dropped instead of writing into the new
// directory's (not-yet-resolved) column, and ColumnResolved reports false in the meantime.
func TestHandlePanelDirChanged_cancelsStaleRunAndRejectsOldWakes(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()

	p := testPanel(t, dir, nil)
	p.Sort = panel.SortState{Mode: panel.SortMeta, MetaColumn: "info"}

	fh := &fakeHost{panels: [2]*panel.State{p}}
	h := &Handler{host: fh, model: &ui.Model{}, config: config.Default()}
	h.activeEntries[0] = []string{"info"}
	h.navPath[0] = filepath.Clean(dir)
	h.model.MetaResults[0] = []ui.MetaColumnState{
		{EntryName: "info", Pending: "*", PendingCount: 1, Results: map[string]string{"/old": "*"}},
	}
	h.runGen[0] = 1
	cancelled := false
	h.cancel[0] = func() { cancelled = true }

	p.Path = testPanel(t, other, nil).Path

	h.HandlePanelDirChanged(0)

	if !cancelled {
		t.Fatal("expected the in-flight run to be cancelled")
	}
	if h.cancel[0] != nil {
		t.Fatal("expected cancel to be cleared after HandlePanelDirChanged")
	}
	if h.ColumnResolved(0) {
		t.Fatal("ColumnResolved = true right after a directory change, want false")
	}

	staleGen := uint64(1)
	if h.runGen[0] == staleGen {
		t.Fatalf("runGen = %d, want it bumped past the stale generation", h.runGen[0])
	}
	h.HandleWake(WakePayload{PanelID: 0, EntryName: "info", Path: "/old", Value: "done", Gen: staleGen})
	h.HandleRenderFlush() // stops the debounce timer HandleWake armed; h.screen is nil in this test
	if got := h.model.MetaResults[0][0].Results["/old"]; got != "*" {
		t.Fatalf("stale-gen wake modified results: got %q, want unchanged %q", got, "*")
	}
}

func TestEntryCmd_whenFiltersDirRows(t *testing.T) {
	cmdDef := metacmds.MetaEntry{
		Name: "films",
		Dirs: "x %f",
		When: []string{"t d & d ^/lib/films$"},
	}
	h := &Handler{}

	cases := []struct {
		name string
		e    localfs.Entry
		dir  string
		want bool
	}{
		{
			name: "dir in matching panel dir",
			e:    localfs.Entry{Name: "harbor", Path: "/lib/films/harbor", Type: localfs.EntryDirectory},
			dir:  "/lib/films",
			want: true,
		},
		{
			name: "dir in non-matching panel dir",
			e:    localfs.Entry{Name: "harbor", Path: "/lib/shows/harbor", Type: localfs.EntryDirectory},
			dir:  "/lib/shows",
			want: false,
		},
		{
			name: "file row has no file command",
			e:    localfs.Entry{Name: "harbor.txt", Path: "/lib/films/harbor.txt", Type: localfs.EntryFile},
			dir:  "/lib/films",
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := h.entryCmd(cmdDef, tc.e, tc.dir)
			if ok != tc.want {
				t.Fatalf("entryCmd() ok = %v, want %v", ok, tc.want)
			}
		})
	}
}

// TestReconcileForPanel_dispatchesOnlyNewEntries covers a rename in a panel with resolved meta
// columns: only the renamed entry's new path is dispatched, other rows keep their values and the
// run generation is unchanged so in-flight results still land.
func TestReconcileForPanel_dispatchesOnlyNewEntries(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)

	dir := t.TempDir()
	harbor := filepath.Join(dir, "harbor.txt")
	meadow := filepath.Join(dir, "meadow.txt")
	thistle := filepath.Join(dir, "thistle.txt")
	p := testPanel(t, dir, []localfs.Entry{
		{Name: "harbor.txt", Path: harbor, Type: localfs.EntryFile},
		{Name: "meadow.txt", Path: meadow, Type: localfs.EntryFile},
	})
	fh := &fakeHost{panels: [2]*panel.State{p}}
	h := &Handler{screen: screen, host: fh, model: &ui.Model{}, config: config.Default()}
	h.cache = map[string]map[string]string{"words": {harbor: "3", meadow: "5"}}

	cmdDef := metacmds.MetaEntry{Name: "words", File: "true", Cache: true}
	h.runForPanel(0, []metacmds.MetaEntry{cmdDef}, []ui.MetaColumnState{{EntryName: "words"}})
	gen := h.runGen[0]

	p.Entries[1] = localfs.Entry{Name: "thistle.txt", Path: thistle, Type: localfs.EntryFile}
	h.ReconcileForPanel(0)

	col := h.model.MetaResults[0][0]
	if col.PendingCount != 1 || col.Results[thistle] != "*" {
		t.Fatalf("PendingCount = %d, thistle = %q; want 1 and running marker", col.PendingCount, col.Results[thistle])
	}
	if col.Results[harbor] != "3" {
		t.Fatalf("harbor = %q, want untouched cached value", col.Results[harbor])
	}
	if h.runGen[0] != gen || h.loadPending[0] {
		t.Fatal("reconcile restarted the run instead of dispatching into it")
	}
}
