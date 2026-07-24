package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
)

func TestAddTransferJobSyncsJobPathMarksWithoutDrainingEvents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	writeFile(t, f1)
	writeFile(t, f2)
	dst := filepath.Join(dir, "dst")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	// Paused jobs stay on the queue so the test does not depend on pollJobEvents / worker timing.
	app.dialogCtrl.AddTransferJob(jobs.TypeCopy, []string{f1}, dst, true, app.dialogCtrl.TransferPreserveFromConfig())
	if n := len(app.model.JobPathMarks); n != 1 {
		t.Fatalf("JobPathMarks len after first enqueue = %d, want 1", n)
	}
	app.dialogCtrl.AddTransferJob(jobs.TypeCopy, []string{f2}, dst, true, app.dialogCtrl.TransferPreserveFromConfig())
	if n := len(app.model.JobPathMarks); n != 2 {
		t.Fatalf("JobPathMarks len after second enqueue = %d, want 2", n)
	}
}
