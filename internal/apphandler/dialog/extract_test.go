package dialog

import (
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/archive"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	previewctrl "github.com/paranoidi/paras-commander/internal/apphandler/preview"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
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
