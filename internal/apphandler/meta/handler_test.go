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
	editedPath string
	editErr    error
	messages   []string
	panels     [2]*panel.State
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
func (f *fakeHost) IconMetaRunning() string           { return "*" }
func (f *fakeHost) OpenFileInExternalEditor(path string) error {
	f.editedPath = path
	return f.editErr
}
func (f *fakeHost) MessageLogWrapCols() int { return 80 }
func (f *fakeHost) AppendTransientMessageLines(banner string, _ []string, _ ui.MessageUrgency) {
	f.messages = append(f.messages, banner)
}
func (f *fakeHost) ClearTransientMessage() {}
func (f *fakeHost) Render()                {}

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

	h := &Handler{screen: screen, model: &ui.Model{}}
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
