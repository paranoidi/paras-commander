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

// runMismatch moves/copies a file "orchard" onto a destination directory of the same name.
func runMismatch(t *testing.T, typ jobs.Type, answers ...jobs.BlockerAnswer) (dstDir string, reqs []jobs.ConflictRequest) {
	t.Helper()
	dir := t.TempDir()
	dstDir = filepath.Join(dir, "meadow")
	if err := os.MkdirAll(filepath.Join(dstDir, "orchard"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "orchard", "pear.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "orchard")
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	transfer := TransferFunc(cfg.Operations, cfg.Jobs, config.CompareConfig{ChunkBytes: 4}, nil, nil, nil)
	job := &jobs.Job{
		ID: jobs.NewJobID(), Type: typ, Status: jobs.StatusRunning,
		Sources:     []pathloc.Path{pathloc.FileMust(src)},
		Destination: pathloc.FileMust(dstDir),
	}
	err := transfer(context.Background(), job, func(jobs.Event) {}, func(r jobs.BlockerRequest) jobs.BlockerAnswer {
		reqs = append(reqs, *r.Conflict)
		if len(answers) == 0 {
			t.Fatalf("unexpected extra prompt")
		}
		a := answers[0]
		answers = answers[1:]
		return a
	})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	return dstDir, reqs
}

func TestMismatchOverwriteAllAndRulesAreReprompted(t *testing.T) {
	t.Parallel()
	for _, typ := range []jobs.Type{jobs.TypeCopy, jobs.TypeMove} {
		for _, first := range []jobs.BlockerAnswer{
			{Decision: jobs.DecisionOverwriteAll},
			rulesAnswer(jobs.ConflictRules{Time: [3]jobs.RuleAction{jobs.RuleOverwrite, jobs.RuleOverwrite, jobs.RuleOverwrite}}, true),
		} {
			dstDir, reqs := runMismatch(t, typ, first, jobs.BlockerAnswer{Decision: jobs.DecisionSkip})
			if len(reqs) != 2 || !reqs[0].Mismatch || !reqs[0].DestIsDir || reqs[1].Reprompt == "" {
				t.Fatalf("%s: want a mismatch prompt then a reprompt, got %+v", typ, reqs)
			}
			if got := readFile(t, filepath.Join(dstDir, "orchard", "pear.txt")); got != "keep" {
				t.Fatalf("%s: directory contents lost: %q", typ, got)
			}
		}
	}
}

func TestMismatchRename(t *testing.T) {
	t.Parallel()
	dstDir, _ := runMismatch(t, jobs.TypeMove, jobs.BlockerAnswer{Decision: jobs.DecisionRename})
	if got := readFile(t, filepath.Join(dstDir, "orchard (1)")); got != "new" {
		t.Fatalf("renamed = %q", got)
	}
	if got := readFile(t, filepath.Join(dstDir, "orchard", "pear.txt")); got != "keep" {
		t.Fatalf("directory contents lost: %q", got)
	}
}
