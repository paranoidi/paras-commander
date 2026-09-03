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

func TestTransferFuncOverwriteAllSameSizeSkipsDifferingSize(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dstDir := filepath.Join(dir, "out")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatal(err)
	}

	matchSrc := filepath.Join(dir, "match.txt")
	diffSrc := filepath.Join(dir, "diff.txt")
	if err := os.WriteFile(matchSrc, []byte("aaaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(diffSrc, []byte("bbbbbbbb"), 0o644); err != nil {
		t.Fatal(err)
	}

	matchDst := filepath.Join(dstDir, "match.txt")
	diffDst := filepath.Join(dstDir, "diff.txt")
	if err := os.WriteFile(matchDst, []byte("orig"), 0o644); err != nil { // same size as matchSrc
		t.Fatal(err)
	}
	if err := os.WriteFile(diffDst, []byte("orig-diff"), 0o644); err != nil { // different size than diffSrc
		t.Fatal(err)
	}

	matchLoc, err := pathloc.File(matchSrc)
	if err != nil {
		t.Fatal(err)
	}
	diffLoc, err := pathloc.File(diffSrc)
	if err != nil {
		t.Fatal(err)
	}
	dstLoc, err := pathloc.File(dstDir)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	transfer := TransferFunc(cfg.Operations, cfg.Jobs)

	job := &jobs.Job{
		ID:          jobs.NewJobID(),
		Type:        jobs.TypeCopy,
		Status:      jobs.StatusRunning,
		Sources:     []pathloc.Path{matchLoc, diffLoc},
		Destination: dstLoc,
	}

	err = transfer(context.Background(), job, func(jobs.Event) {}, func(jobs.BlockerRequest) jobs.ConflictDecision {
		return jobs.DecisionOverwriteAllSameSize
	})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}

	gotMatch, err := os.ReadFile(matchDst)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotMatch) != "aaaa" {
		t.Fatalf("match.txt = %q, want overwritten with source content", gotMatch)
	}

	gotDiff, err := os.ReadFile(diffDst)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotDiff) != "orig-diff" {
		t.Fatalf("diff.txt = %q, want left untouched (skipped)", gotDiff)
	}
}
