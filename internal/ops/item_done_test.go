package ops

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type itemOutcome struct {
	name   string
	res    ItemResult
	reason string
}

func recordItems(out *[]itemOutcome) func(PlanItem, ItemResult, string) {
	return func(item PlanItem, res ItemResult, reason string) {
		*out = append(*out, itemOutcome{filepath.Base(item.Src.String()), res, reason})
	}
}

func outcomesString(out []itemOutcome) string {
	var parts []string
	for _, o := range out {
		s := o.name + "=" + []string{"done", "skipped", "failed"}[o.res]
		if o.reason != "" {
			s += "(" + o.reason + ")"
		}
		parts = append(parts, s)
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

func TestCopyReportsEveryItemOutcome(t *testing.T) {
	srcDir := filepath.Join(t.TempDir(), "orchard")
	dstDir := t.TempDir()
	for _, name := range []string{"acorn.txt", "bramble.txt", "clover.txt"} {
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dstDir, "orchard"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "orchard", "bramble.txt"), []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}
	var got []itemOutcome
	opts := Options{CopyBufferKiB: 4, OnItemDone: recordItems(&got)}
	if _, _, err := ExecuteCopy(context.Background(), MustPaths(srcDir), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, actionResolver(ActionSkip), nil); err != nil {
		t.Fatal(err)
	}
	if s, want := outcomesString(got), "acorn.txt=done bramble.txt=skipped(exists) clover.txt=done"; s != want {
		t.Fatalf("outcomes = %s, want %s", s, want)
	}
}

func TestCopyIdenticalReportsSkipped(t *testing.T) {
	src, dstDir := writeConflictPair(t, "lantern.txt", "same", "same")
	var got []itemOutcome
	opts := Options{CopyBufferKiB: 4, OnItemDone: recordItems(&got)}
	if _, _, err := ExecuteCopy(context.Background(), MustPaths(src), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, actionResolver(ActionIdentical), nil); err != nil {
		t.Fatal(err)
	}
	if s := outcomesString(got); s != "lantern.txt=skipped(identical)" {
		t.Fatalf("outcomes = %s", s)
	}
}

func TestMoveReportsRenameAndSkip(t *testing.T) {
	srcDir, dstDir := t.TempDir(), t.TempDir()
	for _, name := range []string{"willow.bin", "copper.bin"} {
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dstDir, "copper.bin"), []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}
	var got []itemOutcome
	opts := Options{CopyBufferKiB: 4, OnItemDone: recordItems(&got)}
	srcs := MustPaths(filepath.Join(srcDir, "willow.bin"), filepath.Join(srcDir, "copper.bin"))
	if _, _, err := ExecuteMove(context.Background(), srcs, MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, actionResolver(ActionSkip), nil); err != nil {
		t.Fatal(err)
	}
	if s, want := outcomesString(got), "copper.bin=skipped(exists) willow.bin=done"; s != want {
		t.Fatalf("outcomes = %s, want %s", s, want)
	}
}

func TestDeletePathsReportsDone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "thistle.log")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var got []string
	if _, _, err := ExecuteDeletePaths(context.Background(), []string{MustPath(path).String()}, nil, func(item PlanItem, res ItemResult, _ string) {
		if res == ItemDone {
			got = append(got, item.Src.Base())
		}
	}, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "thistle.log" {
		t.Fatalf("deleted = %v", got)
	}
}

func TestCopyReportsItemsProcessedIncludingSkippedDirChildren(t *testing.T) {
	srcDir := filepath.Join(t.TempDir(), "orchard")
	dstDir := t.TempDir()
	for _, rel := range []string{"acorn.txt", "grove/bramble.txt", "grove/clover.txt"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(srcDir, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, rel), []byte(rel), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A file where the grove directory would go: the directory is skipped as a whole.
	if err := os.MkdirAll(filepath.Join(dstDir, "orchard"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "orchard", "grove"), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, _, _, _, err := BuildCopyPlanWithTotalsCtx(context.Background(), MustPaths(srcDir), MustPath(dstDir), PlanBuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	items, dirs, _ := SummarizePlan(plan)
	processed := -1
	opts := Options{CopyBufferKiB: 4, OnItemsProcessed: func(n int) { processed = n }}
	if _, _, err := ExecuteCopy(context.Background(), MustPaths(srcDir), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, actionResolver(ActionSkip), nil); err != nil {
		t.Fatal(err)
	}
	if want := items - dirs; processed != want || want != 3 {
		t.Fatalf("processed = %d, want every file of the plan (%d, expected 3)", processed, want)
	}
}
