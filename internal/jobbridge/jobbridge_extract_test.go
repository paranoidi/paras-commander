package jobbridge

import (
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/archive"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestTransferFuncExtractExistingStreamHonorsBlocker(t *testing.T) {
	requireGzipToolchain(t)

	t.Run("skip keeps existing", func(t *testing.T) {
		src, destDir, existing := extractStreamConflictTree(t, "incoming-thicket", "keep-harbor")
		runExtractTransfer(t, src, destDir, jobs.DecisionSkip)

		got, err := os.ReadFile(existing)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "keep-harbor" {
			t.Fatalf("existing output = %q, want keep-harbor", got)
		}
	})

	t.Run("overwrite replaces existing", func(t *testing.T) {
		src, destDir, existing := extractStreamConflictTree(t, "incoming-thicket", "keep-harbor")
		runExtractTransfer(t, src, destDir, jobs.DecisionOverwrite)

		got, err := os.ReadFile(existing)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "incoming-thicket" {
			t.Fatalf("existing output = %q, want incoming-thicket", got)
		}
	})
}

func runExtractTransfer(t *testing.T, src, destDir string, decision jobs.ConflictDecision) {
	t.Helper()
	srcLoc, err := pathloc.File(src)
	if err != nil {
		t.Fatal(err)
	}
	dstLoc, err := pathloc.File(destDir)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	transfer := TransferFunc(cfg.Operations, cfg.Jobs, cfg.Dedup.ChunkBytes, nil)
	job := &jobs.Job{
		ID:          jobs.NewJobID(),
		Type:        jobs.TypeExtract,
		Status:      jobs.StatusRunning,
		Sources:     []pathloc.Path{srcLoc},
		Destination: dstLoc,
	}

	var blockers int
	err = transfer(context.Background(), job, func(jobs.Event) {}, func(req jobs.BlockerRequest) jobs.BlockerAnswer {
		blockers++
		if req.Kind != jobs.BlockerKindConflict {
			t.Fatalf("blocker kind = %q, want %q", req.Kind, jobs.BlockerKindConflict)
		}
		if !req.Conflict.NoCompare {
			t.Fatal("extract conflict must set NoCompare")
		}
		return jobs.BlockerAnswer{Decision: decision}
	})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if blockers != 1 {
		t.Fatalf("conflict blockers = %d, want 1", blockers)
	}
}

func extractStreamConflictTree(t *testing.T, archiveBody, existingBody string) (src, destDir, existing string) {
	t.Helper()
	srcDir := t.TempDir()
	destDir = t.TempDir()
	src = filepath.Join(srcDir, "meadow.gz")
	writeGzipFile(t, src, archiveBody)
	existing = filepath.Join(destDir, "meadow")
	if err := os.WriteFile(existing, []byte(existingBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return src, destDir, existing
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

func requireGzipToolchain(t *testing.T) {
	t.Helper()
	if archive.ProbeToolchain().Gzip == "" {
		t.Skip("gzip not in PATH")
	}
}
