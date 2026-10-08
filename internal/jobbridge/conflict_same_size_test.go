package jobbridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

type conflictCase struct {
	srcData, dstData string
	srcAge, dstAge   time.Duration // how long ago each file was modified
}

// runConflict transfers one colliding file ("harbor.txt") and returns the source and destination
// paths plus the blocker requests seen. answers are consumed one per prompt.
func runConflict(t *testing.T, typ jobs.Type, c conflictCase, answers ...jobs.ConflictDecision) (src, dstDir string, reqs []jobs.ConflictRequest) {
	t.Helper()
	dir := t.TempDir()
	dstDir = filepath.Join(dir, "meadow")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src = filepath.Join(dir, "harbor.txt")
	dst := filepath.Join(dstDir, "harbor.txt")
	now := time.Now()
	for _, f := range []struct {
		path, data string
		age        time.Duration
	}{{src, c.srcData, c.srcAge}, {dst, c.dstData, c.dstAge}} {
		if err := os.WriteFile(f.path, []byte(f.data), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(f.path, now.Add(-f.age), now.Add(-f.age)); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Default()
	transfer := TransferFunc(cfg.Operations, cfg.Jobs, 4, nil) // tiny chunks exercise the chunk loop
	job := &jobs.Job{
		ID:          jobs.NewJobID(),
		Type:        typ,
		Status:      jobs.StatusRunning,
		Sources:     []pathloc.Path{pathloc.FileMust(src)},
		Destination: pathloc.FileMust(dstDir),
	}
	err := transfer(context.Background(), job, func(jobs.Event) {}, func(r jobs.BlockerRequest) jobs.ConflictDecision {
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
	return src, dstDir, reqs
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestConflictRules(t *testing.T) {
	t.Parallel()
	const (
		stale  = 2 * time.Hour
		recent = time.Hour
	)
	tests := []struct {
		name      string
		rule      jobs.ConflictDecision
		c         conflictCase
		overwrite bool
	}{
		{"newer yes", jobs.DecisionOverwriteIfNewer, conflictCase{"aaaa", "bbbb", recent, stale}, true},
		{"newer no", jobs.DecisionOverwriteIfNewer, conflictCase{"aaaa", "bbbb", stale, recent}, false},
		{"older yes", jobs.DecisionOverwriteIfOlder, conflictCase{"aaaa", "bbbb", stale, recent}, true},
		{"older no", jobs.DecisionOverwriteIfOlder, conflictCase{"aaaa", "bbbb", recent, stale}, false},
		{"smaller yes", jobs.DecisionOverwriteIfExistingSmaller, conflictCase{"aaaaaa", "bb", stale, stale}, true},
		{"smaller no", jobs.DecisionOverwriteIfExistingSmaller, conflictCase{"aa", "bbbbbb", stale, stale}, false},
		{"differs yes", jobs.DecisionOverwriteIfSizeDiffers, conflictCase{"aaaa", "bbbbbb", stale, stale}, true},
		{"differs no", jobs.DecisionOverwriteIfSizeDiffers, conflictCase{"aaaa", "bbbb", stale, stale}, false},
		{"same size yes", jobs.DecisionOverwriteIfSameSize, conflictCase{"aaaa", "bbbb", stale, stale}, true},
		{"same size no", jobs.DecisionOverwriteIfSameSize, conflictCase{"aaaa", "bbbbbb", stale, stale}, false},
	}
	for _, tt := range tests {
		for _, all := range []bool{false, true} {
			rule, name := tt.rule, tt.name
			if all {
				rule, name = rule.All(), name+" all"
			}
			t.Run(name, func(t *testing.T) {
				_, dstDir, reqs := runConflict(t, jobs.TypeCopy, tt.c, rule)
				want := tt.c.dstData
				if tt.overwrite {
					want = tt.c.srcData
				}
				if got := readFile(t, filepath.Join(dstDir, "harbor.txt")); got != want {
					t.Fatalf("dest = %q, want %q", got, want)
				}
				if len(reqs) != 1 {
					t.Fatalf("prompts = %d, want 1", len(reqs))
				}
			})
		}
	}
}

func TestConflictKeepBoth(t *testing.T) {
	t.Parallel()
	_, dstDir, _ := runConflict(t, jobs.TypeCopy, conflictCase{"aaaa", "bbbb", time.Hour, time.Hour}, jobs.DecisionKeepBoth)
	if got := readFile(t, filepath.Join(dstDir, "harbor.txt")); got != "bbbb" {
		t.Fatalf("existing = %q, want untouched", got)
	}
	if got := readFile(t, filepath.Join(dstDir, "harbor (1).txt")); got != "aaaa" {
		t.Fatalf("copy = %q, want source content", got)
	}
}

func TestConflictCompareIdentical(t *testing.T) {
	t.Parallel()
	same := conflictCase{"0123456789", "0123456789", time.Hour, time.Hour}
	src, dstDir, reqs := runConflict(t, jobs.TypeCopy, same, jobs.DecisionCompare)
	if len(reqs) != 1 {
		t.Fatalf("prompts = %d, want 1", len(reqs))
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("copy must keep the source: %v", err)
	}
	if got := readFile(t, filepath.Join(dstDir, "harbor.txt")); got != "0123456789" {
		t.Fatalf("dest = %q", got)
	}

	src, dstDir, _ = runConflict(t, jobs.TypeMove, same, jobs.DecisionCompare)
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("move of an identical file must remove the source, stat err = %v", err)
	}
	if got := readFile(t, filepath.Join(dstDir, "harbor.txt")); got != "0123456789" {
		t.Fatalf("dest = %q", got)
	}
}

func TestConflictCompareDifferentPromptsAgain(t *testing.T) {
	t.Parallel()
	// Same size, differing in the last chunk (chunk size 4 in runConflict).
	diff := conflictCase{"0123456789", "012345678X", time.Hour, time.Hour}
	_, dstDir, reqs := runConflict(t, jobs.TypeCopy, diff, jobs.DecisionCompare, jobs.DecisionOverwrite)
	if len(reqs) != 2 || reqs[0].ContentDiffers || !reqs[1].ContentDiffers {
		t.Fatalf("prompts = %+v, want a second prompt with ContentDiffers", reqs)
	}
	if got := readFile(t, filepath.Join(dstDir, "harbor.txt")); got != "0123456789" {
		t.Fatalf("dest = %q, want overwritten", got)
	}

	// Different sizes also prompt again, and Skip leaves the destination alone.
	diff = conflictCase{"0123456789", "short", time.Hour, time.Hour}
	_, dstDir, reqs = runConflict(t, jobs.TypeCopy, diff, jobs.DecisionCompare, jobs.DecisionSkip)
	if len(reqs) != 2 || !reqs[1].ContentDiffers {
		t.Fatalf("prompts = %+v", reqs)
	}
	if got := readFile(t, filepath.Join(dstDir, "harbor.txt")); got != "short" {
		t.Fatalf("dest = %q, want untouched", got)
	}
}
