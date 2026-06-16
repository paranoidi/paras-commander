package jobbridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestTransferFuncUsesJobPreserveOptions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "secret.txt")
	dstDir := filepath.Join(dir, "out")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	srcLoc, err := pathloc.File(srcFile)
	if err != nil {
		t.Fatal(err)
	}
	dstLoc, err := pathloc.File(dstDir)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Operations.PreservePermissions = true
	transfer := TransferFunc(cfg.Operations, cfg.Jobs)

	job := &jobs.Job{
		ID:                  jobs.NewJobID(),
		Type:                jobs.TypeCopy,
		Status:              jobs.StatusRunning,
		Sources:             []pathloc.Path{srcLoc},
		Destination:         dstLoc,
		PreservePermissions: false,
		PreserveTimestamps:  false,
	}

	err = transfer(context.Background(), job, func(jobs.Event) {}, func(jobs.BlockerRequest) jobs.ConflictDecision {
		return jobs.DecisionOverwrite
	})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}

	dstFile := filepath.Join(dstDir, "secret.txt")
	info, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("stat dest: %v", err)
	}
	if info.Mode().Perm() == 0o600 {
		t.Fatalf("dest mode = %o, want preserved permissions disabled (not 0600)", info.Mode().Perm())
	}
}
