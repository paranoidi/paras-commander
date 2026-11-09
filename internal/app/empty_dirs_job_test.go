package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// Confirming the left-empty prompt queues the rmdir-only job: a file that arrived in a directory
// while the prompt was open survives, and only what is still empty goes.
func TestDanglingPromptNeverDeletesAFileThatArrived(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	busy, hollow := filepath.Join(root, "harbor"), filepath.Join(root, "meadow")
	for _, d := range []string{busy, hollow} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.dialogCtrl.PromptDanglingDirDelete([]string{busy, hollow})
	fd := app.model.FileDialog
	if !fd.Open || fd.DialogType != dialog.FileDialogDelete || !fd.DeleteDanglingDirs {
		t.Fatalf("prompt = %+v", fd)
	}

	download := filepath.Join(busy, "download.part")
	if err := os.WriteFile(download, []byte("still arriving"), 0o644); err != nil {
		t.Fatal(err)
	}
	app.dialogCtrl.ExecuteDelete()
	all := app.jobState.AllJobs()
	if len(all) != 1 || !all[0].DeleteEmptyDirsOnly {
		t.Fatalf("jobs = %+v", all)
	}
	flushBackgroundJobs(t, app)

	if _, err := os.Stat(download); err != nil {
		t.Fatalf("the file that arrived went: %v", err)
	}
	if _, err := os.Stat(hollow); !os.IsNotExist(err) {
		t.Fatalf("the empty directory stayed: %v", err)
	}
	j := app.jobState.AllJobs()[0]
	if j.Ops == nil {
		t.Fatal("no operation log")
	}
	if st := j.Ops.Stats(); st.Skipped != 1 || st.Done != 1 {
		t.Fatalf("operation stats = %+v, want one done and one skipped", st)
	}
}
