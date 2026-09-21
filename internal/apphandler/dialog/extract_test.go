package dialog

import (
	"compress/gzip"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	previewctrl "github.com/paranoidi/paras-commander/internal/apphandler/preview"
	"github.com/paranoidi/paras-commander/internal/archive"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/uitest"
)

func TestExtractQueuedMessageDoesNotCallExistingUnsupportedTool(t *testing.T) {
	t.Parallel()
	msg := extractQueuedMessage(1, []string{`meadow.gz: output "meadow" already exists`})
	if strings.Contains(msg, "unsupported") || strings.Contains(msg, "missing tool") {
		t.Fatalf("toast = %q, must not call existing/colliding output an unsupported tool", msg)
	}
	if !strings.Contains(msg, "Extract queued (1 archive)") {
		t.Fatalf("toast = %q, want queued archive count", msg)
	}

	collide := extractQueuedMessage(2, []string{`tulip.gz: output "meadow" collides with meadow.gz`})
	if strings.Contains(collide, "unsupported") || strings.Contains(collide, "missing tool") {
		t.Fatalf("toast = %q, must not call colliding output an unsupported tool", collide)
	}
}

func TestExecuteExtractQueuesExistingStreamWithoutUnsupportedToast(t *testing.T) {
	if archive.ProbeToolchain().Gzip == "" {
		t.Skip("gzip not in PATH")
	}

	srcDir := t.TempDir()
	destDir := t.TempDir()
	archivePath := filepath.Join(srcDir, "meadow.gz")
	writeGzipFile(t, archivePath, "incoming-thicket")
	if err := os.WriteFile(filepath.Join(destDir, "meadow"), []byte("keep-harbor"), 0o644); err != nil {
		t.Fatal(err)
	}

	model := &ui.Model{}
	if err := model.Primary.Load(srcDir); err != nil {
		t.Fatalf("Load primary: %v", err)
	}
	if err := model.Secondary.Load(destDir); err != nil {
		t.Fatalf("Load secondary: %v", err)
	}

	cfg := config.Default()
	cfg.Preview.Prefetch = false
	host := &identityTestHost{model: model, cfg: cfg}
	screen := uitest.Screen(t, 80, 24)
	jobState := jobs.NewState()
	h := New(Deps{
		Host:   host,
		Screen: screen,
		Model:  model,
		Jobs: jobsctrl.New(jobsctrl.Deps{
			Host: host, Screen: screen, Model: model, State: jobState, Config: cfg,
		}),
		Preview: previewctrl.New(previewctrl.Deps{
			Host: host, Screen: screen, Model: model, Ctx: context.Background(),
		}),
	})

	h.model.FileDialog = uidialog.FileDialogState{
		Open:         true,
		DialogType:   uidialog.FileDialogExtract,
		FocusedField: 0,
		Fields: []uidialog.FileDialogField{{
			Label: "Destination",
			Value: destDir,
		}},
		ExtractSources: []string{archivePath},
	}

	h.ExecuteExtract()

	snap := jobState.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("jobs = %d, want 1 extract job for the existing stream target", len(snap))
	}
	if snap[0].Type != jobs.TypeExtract {
		t.Fatalf("job type = %s, want extract", snap[0].Type)
	}
	if len(snap[0].Sources) != 1 || snap[0].Sources[0].String() != archivePath {
		t.Fatalf("job sources = %v, want [%s]", snap[0].Sources, archivePath)
	}

	if len(host.messages) == 0 {
		t.Fatal("expected enqueue toast")
	}
	toast := host.messages[len(host.messages)-1]
	if strings.Contains(toast, "unsupported") || strings.Contains(toast, "missing tool") {
		t.Fatalf("toast = %q, must not call existing stream output an unsupported tool", toast)
	}
}

func TestRemoteExtractProbeQueuesExistingStreamWithoutErrorDialog(t *testing.T) {
	if archive.ProbeToolchain().Gzip == "" {
		t.Skip("gzip not in PATH")
	}

	srcDir := t.TempDir()
	destDir := t.TempDir()
	archivePath := filepath.Join(srcDir, "meadow.gz")
	writeGzipFile(t, archivePath, "incoming-thicket")
	if err := os.WriteFile(filepath.Join(destDir, "meadow"), []byte("keep-harbor"), 0o644); err != nil {
		t.Fatal(err)
	}

	h, host, jobState, screen := newExtractProbeHarness(t, srcDir, destDir)
	h.testRemote = extractRemoteBackend{}
	h.model.FileDialog = uidialog.FileDialogState{
		Open:         true,
		DialogType:   uidialog.FileDialogExtract,
		FocusedField: 0,
		Fields: []uidialog.FileDialogField{{
			Label: "Destination",
			Value: destDir,
		}},
		ExtractSources: []string{archivePath},
	}

	h.ExecuteExtract()
	h.ApplyRemoteFileOp(waitRemoteFileOpPayload(t, screen))

	assertExtractQueuedWithoutErrorDialog(t, host, jobState, archivePath)
}

func TestApplyRemoteExtractProbeQueuesKeepExistingWithoutErrorDialog(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	destDir := t.TempDir()
	archivePath := filepath.Join(srcDir, "tulip.gz")
	if err := os.WriteFile(archivePath, []byte("keep-harbor"), 0o644); err != nil {
		t.Fatal(err)
	}

	h, host, jobState, _ := newExtractProbeHarness(t, srcDir, destDir)
	h.remoteFileOpGen = 1
	h.ApplyRemoteFileOp(RemoteFileOpPayload{
		Gen:  1,
		Kind: RemoteFileOpExtractProbe,
		Err:  &ops.Error{Op: "extract", Text: "no supported archives selected"},
		extract: extractProbeApply{
			sources: []string{archivePath},
			dest:    destDir,
			skipped: []string{`tulip.gz: output "tulip" already exists`},
		},
	})

	assertExtractQueuedWithoutErrorDialog(t, host, jobState, archivePath)
}

type extractMessageHost struct {
	identityTestHost
	messageDialogs []string
}

func (f *extractMessageHost) OpenMessageDialog(title, message string) {
	f.messageDialogs = append(f.messageDialogs, title+": "+message)
}

func newExtractProbeHarness(t *testing.T, srcDir, destDir string) (*Handler, *extractMessageHost, *jobs.State, tcell.SimulationScreen) {
	t.Helper()
	model := &ui.Model{}
	if err := model.Primary.Load(srcDir); err != nil {
		t.Fatalf("Load primary: %v", err)
	}
	if err := model.Secondary.Load(destDir); err != nil {
		t.Fatalf("Load secondary: %v", err)
	}
	cfg := config.Default()
	cfg.Preview.Prefetch = false
	host := &extractMessageHost{identityTestHost: identityTestHost{model: model, cfg: cfg}}
	screen := uitest.Screen(t, 80, 24)
	jobState := jobs.NewState()
	h := New(Deps{
		Host:   host,
		Screen: screen,
		Model:  model,
		Jobs: jobsctrl.New(jobsctrl.Deps{
			Host: host, Screen: screen, Model: model, State: jobState, Config: cfg,
		}),
		Preview: previewctrl.New(previewctrl.Deps{
			Host: host, Screen: screen, Model: model, Ctx: context.Background(),
		}),
	})
	return h, host, jobState, screen
}

func assertExtractQueuedWithoutErrorDialog(t *testing.T, host *extractMessageHost, jobState *jobs.State, archivePath string) {
	t.Helper()
	for _, dlg := range host.messageDialogs {
		if strings.HasPrefix(dlg, "Extract:") {
			t.Fatalf("Extract error dialog = %q, want job queued", dlg)
		}
	}
	snap := jobState.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("jobs = %d, want 1 extract job for the existing stream target", len(snap))
	}
	if snap[0].Type != jobs.TypeExtract {
		t.Fatalf("job type = %s, want extract", snap[0].Type)
	}
	if len(snap[0].Sources) != 1 || snap[0].Sources[0].String() != archivePath {
		t.Fatalf("job sources = %v, want [%s]", snap[0].Sources, archivePath)
	}
}

func waitRemoteFileOpPayload(t *testing.T, screen tcell.SimulationScreen) RemoteFileOpPayload {
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
		p, ok := ie.Data().(RemoteFileOpPayload)
		if !ok {
			continue
		}
		return p
	}
	t.Fatal("timeout waiting for RemoteFileOpPayload")
	return RemoteFileOpPayload{}
}

type extractRemoteBackend struct{}

func (extractRemoteBackend) Scheme() pathloc.Scheme { return pathloc.SchemeSFTP }
func (extractRemoteBackend) Stat(context.Context, pathloc.Path) (fsbackend.Entry, error) {
	return fsbackend.Entry{}, os.ErrNotExist
}
func (extractRemoteBackend) List(context.Context, pathloc.Path) ([]fsbackend.Entry, error) {
	return nil, nil
}
func (extractRemoteBackend) Mkdir(context.Context, pathloc.Path, fs.FileMode) error { return nil }
func (extractRemoteBackend) Rename(context.Context, pathloc.Path, pathloc.Path) error {
	return fs.ErrInvalid
}
func (extractRemoteBackend) OpenRead(context.Context, pathloc.Path) (io.ReadCloser, error) {
	return nil, fs.ErrInvalid
}
func (extractRemoteBackend) OpenWrite(context.Context, pathloc.Path, int64, fsbackend.CreateOpts) (io.WriteCloser, error) {
	return nil, fs.ErrInvalid
}
func (extractRemoteBackend) Remove(context.Context, pathloc.Path) error { return fs.ErrInvalid }
func (extractRemoteBackend) ReadSymlink(context.Context, pathloc.Path) (string, error) {
	return "", fs.ErrInvalid
}
func (extractRemoteBackend) Symlink(context.Context, pathloc.Path, string) error {
	return fs.ErrInvalid
}

func writeGzipFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := gzip.NewWriter(f)
	if _, err := w.Write([]byte(content)); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
