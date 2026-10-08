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
func runConflict(t *testing.T, typ jobs.Type, c conflictCase, answers ...jobs.BlockerAnswer) (src, dstDir string, reqs []jobs.ConflictRequest) {
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

func rulesAnswer(r jobs.ConflictRules, all bool) jobs.BlockerAnswer {
	d := jobs.DecisionRules
	if all {
		d = d.All()
	}
	return jobs.BlockerAnswer{Decision: d, Rules: &r}
}

func TestConflictRules(t *testing.T) {
	t.Parallel()
	const (
		stale  = 2 * time.Hour
		recent = time.Hour
	)
	tests := []struct {
		name      string
		rules     jobs.ConflictRules
		c         conflictCase
		overwrite bool
	}{
		{"newer yes", jobs.ConflictRules{Time: [3]jobs.RuleAction{jobs.TimeDestOlder: jobs.RuleOverwrite}}, conflictCase{"aaaa", "bbbb", recent, stale}, true},
		{"newer no", jobs.ConflictRules{Time: [3]jobs.RuleAction{jobs.TimeDestOlder: jobs.RuleOverwrite}}, conflictCase{"aaaa", "bbbb", stale, recent}, false},
		{"smaller yes", jobs.ConflictRules{Size: [3]jobs.RuleAction{jobs.SizeDestSmaller: jobs.RuleOverwrite}}, conflictCase{"aaaaaa", "bb", stale, stale}, true},
		{"smaller no", jobs.ConflictRules{Size: [3]jobs.RuleAction{jobs.SizeDestSmaller: jobs.RuleOverwrite}}, conflictCase{"aa", "bbbbbb", stale, stale}, false},
		{"same size yes", jobs.ConflictRules{Size: [3]jobs.RuleAction{jobs.SizeSame: jobs.RuleOverwrite}}, conflictCase{"aaaa", "bbbb", stale, stale}, true},
		{"same size no", jobs.ConflictRules{Size: [3]jobs.RuleAction{jobs.SizeSame: jobs.RuleOverwrite}}, conflictCase{"aaaa", "bbbbbb", stale, stale}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Rules that do not match leave the file on Ask, which prompts again: answer Skip.
			_, dstDir, reqs := runConflict(t, jobs.TypeCopy, tt.c, rulesAnswer(tt.rules, false), jobs.BlockerAnswer{Decision: jobs.DecisionSkip})
			want := tt.c.dstData
			if tt.overwrite {
				want = tt.c.srcData
			}
			if got := readFile(t, filepath.Join(dstDir, "harbor.txt")); got != want {
				t.Fatalf("dest = %q, want %q", got, want)
			}
			if wantPrompts := map[bool]int{true: 1, false: 2}[tt.overwrite]; len(reqs) != wantPrompts {
				t.Fatalf("prompts = %d, want %d", len(reqs), wantPrompts)
			}
			if !tt.overwrite && reqs[1].Reprompt == "" {
				t.Fatal("unmatched file must reprompt with a note")
			}
		})
	}
}

func TestConflictRulesTimeBeatsSize(t *testing.T) {
	t.Parallel()
	rules := jobs.ConflictRules{
		Time: [3]jobs.RuleAction{jobs.TimeDestOlder: jobs.RuleSkip},
		Size: [3]jobs.RuleAction{jobs.SizeDestSmaller: jobs.RuleSkip},
	}
	_, dstDir, reqs := runConflict(t, jobs.TypeCopy, conflictCase{"aaaaaa", "bb", time.Hour, 2 * time.Hour}, rulesAnswer(rules, false))
	if len(reqs) != 1 || readFile(t, filepath.Join(dstDir, "harbor.txt")) != "bb" {
		t.Fatalf("expected a single prompt and skipped file, prompts=%d", len(reqs))
	}
}

func TestConflictKeepBoth(t *testing.T) {
	t.Parallel()
	rules := jobs.ConflictRules{Size: [3]jobs.RuleAction{jobs.SizeSame: jobs.RuleKeepBoth}}
	_, dstDir, _ := runConflict(t, jobs.TypeCopy, conflictCase{"aaaa", "bbbb", time.Hour, time.Hour}, rulesAnswer(rules, false))
	if got := readFile(t, filepath.Join(dstDir, "harbor.txt")); got != "bbbb" {
		t.Fatalf("existing = %q, want untouched", got)
	}
	if got := readFile(t, filepath.Join(dstDir, "harbor (1).txt")); got != "aaaa" {
		t.Fatalf("copy = %q, want source content", got)
	}
}

func TestConflictSkipIdentical(t *testing.T) {
	t.Parallel()
	same := conflictCase{"0123456789", "0123456789", time.Hour, time.Hour}
	rules := jobs.ConflictRules{SkipIdentical: true}
	src, dstDir, reqs := runConflict(t, jobs.TypeCopy, same, rulesAnswer(rules, false))
	if len(reqs) != 1 {
		t.Fatalf("prompts = %d, want 1", len(reqs))
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("copy must keep the source: %v", err)
	}
	if got := readFile(t, filepath.Join(dstDir, "harbor.txt")); got != "0123456789" {
		t.Fatalf("dest = %q", got)
	}

	src, dstDir, _ = runConflict(t, jobs.TypeMove, same, rulesAnswer(rules, false))
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("move of an identical file must remove the source, stat err = %v", err)
	}
	if got := readFile(t, filepath.Join(dstDir, "harbor.txt")); got != "0123456789" {
		t.Fatalf("dest = %q", got)
	}
}

func TestConflictSkipIdenticalDifferentFallsToRows(t *testing.T) {
	t.Parallel()
	// Same size, differing in the last chunk (chunk size 4 in runConflict): the rows decide.
	rules := jobs.ConflictRules{SkipIdentical: true, Size: [3]jobs.RuleAction{jobs.SizeSame: jobs.RuleOverwrite}}
	diff := conflictCase{"0123456789", "012345678X", time.Hour, time.Hour}
	_, dstDir, reqs := runConflict(t, jobs.TypeCopy, diff, rulesAnswer(rules, false))
	if len(reqs) != 1 {
		t.Fatalf("prompts = %d, want 1", len(reqs))
	}
	if got := readFile(t, filepath.Join(dstDir, "harbor.txt")); got != "0123456789" {
		t.Fatalf("dest = %q, want overwritten", got)
	}

	// With every row on Ask the file is asked about again.
	_, dstDir, reqs = runConflict(t, jobs.TypeCopy, diff, rulesAnswer(jobs.ConflictRules{SkipIdentical: true}, false), jobs.BlockerAnswer{Decision: jobs.DecisionSkip})
	if len(reqs) != 2 || reqs[1].Reprompt == "" {
		t.Fatalf("prompts = %+v, want a reprompt", reqs)
	}
	if got := readFile(t, filepath.Join(dstDir, "harbor.txt")); got != "012345678X" {
		t.Fatalf("dest = %q, want untouched", got)
	}
}
