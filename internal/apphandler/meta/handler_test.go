package meta

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// fakeHost is a minimal Host stub for handler-logic tests that don't need a real *App.
type fakeHost struct {
	editedPath string
	editErr    error
	messages   []string
}

func (f *fakeHost) SetTransientMessage(text string, _ ui.MessageUrgency) {
	f.messages = append(f.messages, text)
}
func (f *fakeHost) SetErrorMessage(_ string, _ error) {}
func (f *fakeHost) PanelByID(int) *panel.State        { return nil }
func (f *fakeHost) SymbolMetaRunning() string         { return "*" }
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
