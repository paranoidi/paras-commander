package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
)

func TestPathEqualOrUnder(t *testing.T) {
	t.Parallel()
	root := filepath.Join("/tmp", "example", "foo")
	if !pathEqualOrUnder(root, root) {
		t.Fatal("root should match itself")
	}
	child := filepath.Join(root, "bar", "asdf")
	if !pathEqualOrUnder(root, child) {
		t.Fatal("child under root")
	}
	if pathEqualOrUnder(root, filepath.Join("/tmp", "example", "other")) {
		t.Fatal("sibling path should not match")
	}
	if pathEqualOrUnder(filepath.Join("/tmp", "example", "foobar"), filepath.Join("/tmp", "example", "foo")) {
		t.Fatal("prefix segment boundary: foobar is not under foo")
	}
}

func TestEntryPathMarkedByJobs_finishedIgnored(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "x")
	j := JobEntry{
		Status:      string(jobs.StatusCompleted),
		Sources:     []string{root},
		Destination: t.TempDir(),
	}
	if EntryPathMarkedByJobs(child, []JobEntry{j}) {
		t.Fatal("finished job should not mark")
	}
}

func TestEntryPathMarkedByJobs_sourceSubtree(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	root := filepath.Join(tmp, "foo")
	if err := os.MkdirAll(filepath.Join(root, "bar", "asdf"), 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(tmp, "dst")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	j := JobEntry{
		Status:      string(jobs.StatusQueued),
		Sources:     []string{root},
		Destination: dst,
		DestIsDir:   true,
	}
	if !EntryPathMarkedByJobs(root, []JobEntry{j}) {
		t.Fatal("source root")
	}
	if !EntryPathMarkedByJobs(filepath.Join(root, "bar", "asdf"), []JobEntry{j}) {
		t.Fatal("nested under source")
	}
	if EntryPathMarkedByJobs(filepath.Join(tmp, "other"), []JobEntry{j}) {
		t.Fatal("outside tree")
	}
}

func TestEntryPathMarkedByJobs_destinationSubtree(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	src := filepath.Join(tmp, "proj")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	dstParent := filepath.Join(tmp, "out")
	if err := os.MkdirAll(dstParent, 0o755); err != nil {
		t.Fatal(err)
	}
	// ResolveDestination joins basename(src) under dstParent.
	dstRoot := filepath.Join(dstParent, filepath.Base(src))
	if err := os.MkdirAll(filepath.Join(dstRoot, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	j := JobEntry{
		Status:      string(jobs.StatusRunning),
		Sources:     []string{src},
		Destination: dstParent,
		DestIsDir:   true,
	}
	if !EntryPathMarkedByJobs(dstRoot, []JobEntry{j}) {
		t.Fatal("resolved dest root")
	}
	if !EntryPathMarkedByJobs(filepath.Join(dstRoot, "nested"), []JobEntry{j}) {
		t.Fatal("under resolved dest root")
	}
	if EntryPathMarkedByJobs(filepath.Join(dstParent, "unrelated"), []JobEntry{j}) {
		t.Fatal("sibling under dst parent not part of this job")
	}
}

func TestEntryPathJobMarkStatus_moveListedBeforeDeleteNestedChildPrefersDelete(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	parent := filepath.Join(tmp, "proj")
	child := filepath.Join(parent, "nested")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(tmp, "out")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	move := JobEntry{
		Type:        string(jobs.TypeMove),
		Status:      string(jobs.StatusQueued),
		Sources:     []string{parent},
		Destination: dst,
		DestIsDir:   true,
	}
	del := JobEntry{
		Type:     string(jobs.TypeDelete),
		Status:   string(jobs.StatusQueued),
		Sources:  []string{child},
	}
	// Same ordering issue as user report: transfer job appears first in JobsList.
	list := []JobEntry{move, del}
	marked, st := EntryPathJobMarkStatus(child, list)
	if !marked {
		t.Fatal("expected child path marked")
	}
	if st != string(jobs.StatusQueued) {
		t.Fatalf("status = %q, want queued (delete job wins over ancestor move)", st)
	}
}

func TestEntryPathJobMarkStatus_finishedDeleteQueuedMoveOverlappingUsesMove(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	parent := filepath.Join(tmp, "proj")
	child := filepath.Join(parent, "nested")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(tmp, "out")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	// Mirrors AllJobs(): queued jobs first, then finished archive.
	move := JobEntry{
		Type:        string(jobs.TypeMove),
		Status:      string(jobs.StatusQueued),
		Sources:     []string{parent},
		Destination: dst,
		DestIsDir:   true,
	}
	delDone := JobEntry{
		Type:     string(jobs.TypeDelete),
		Status:   string(jobs.StatusCompleted),
		Sources:  []string{child},
	}
	list := []JobEntry{move, delDone}
	marked, st := EntryPathJobMarkStatus(child, list)
	if !marked {
		t.Fatal("expected child path still marked by queued move (ancestor source)")
	}
	if st != string(jobs.StatusQueued) {
		t.Fatalf("status = %q, want queued from move job", st)
	}
}

func TestEntryPathJobMarkStatus_twoMovesSameTypeMoreSpecificSourceWins(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	parent := filepath.Join(tmp, "tree")
	child := filepath.Join(parent, "sub")
	deep := filepath.Join(child, "leaf")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	dstA := filepath.Join(tmp, "outA")
	dstB := filepath.Join(tmp, "outB")
	if err := os.MkdirAll(dstA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dstB, 0o755); err != nil {
		t.Fatal(err)
	}
	moveWide := JobEntry{
		Type:        string(jobs.TypeMove),
		Status:      string(jobs.StatusPaused),
		Sources:     []string{parent},
		Destination: dstA,
		DestIsDir:   true,
	}
	moveNarrow := JobEntry{
		Type:        string(jobs.TypeMove),
		Status:      string(jobs.StatusQueued),
		Sources:     []string{child},
		Destination: dstB,
		DestIsDir:   true,
	}
	// Wider job first; row is under the narrower source only.
	list := []JobEntry{moveWide, moveNarrow}
	marked, st := EntryPathJobMarkStatus(deep, list)
	if !marked {
		t.Fatal("expected row marked")
	}
	if st != string(jobs.StatusQueued) {
		t.Fatalf("status = %q, want queued (narrower move source should win)", st)
	}
}

func TestEntryPathJobMarkStatus_copyListedBeforeMoveSameSubtreePrefersMove(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(filepath.Join(src, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	dstA := filepath.Join(tmp, "da")
	dstB := filepath.Join(tmp, "db")
	if err := os.MkdirAll(dstA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dstB, 0o755); err != nil {
		t.Fatal(err)
	}
	copyJ := JobEntry{
		Type:        string(jobs.TypeCopy),
		Status:      string(jobs.StatusQueued),
		Sources:     []string{src},
		Destination: dstA,
		DestIsDir:   true,
	}
	moveJ := JobEntry{
		Type:        string(jobs.TypeMove),
		Status:      string(jobs.StatusPaused),
		Sources:     []string{src},
		Destination: dstB,
		DestIsDir:   true,
	}
	row := filepath.Join(src, "x")
	list := []JobEntry{copyJ, moveJ}
	marked, st := EntryPathJobMarkStatus(row, list)
	if !marked {
		t.Fatal("expected row marked")
	}
	if st != string(jobs.StatusPaused) {
		t.Fatalf("status = %q, want paused (move beats copy at same specificity)", st)
	}
}

// JobEntriesFromJobs is what the browser passes into drawPanel as JobsList; both
// queued delete and move remain present after enqueue—marker logic must not imply
// the delete row vanished from the queue.
func TestEntryPathJobMarkStatus_jobEntriesFromJobsKeepsBothQueuedJobs(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	parent := filepath.Join(tmp, "proj")
	child := filepath.Join(parent, "nested")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(tmp, "out")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	move := &jobs.Job{
		ID:          "job-move",
		Type:        jobs.TypeMove,
		Status:      jobs.StatusQueued,
		Sources:     []string{parent},
		Destination: dst,
		DestIsDir:   true,
	}
	del := &jobs.Job{
		ID:         "job-del",
		Type:       jobs.TypeDelete,
		Status:     jobs.StatusQueued,
		Sources:    []string{child},
		TotalFiles: 1,
	}
	// Same slice order as AllJobs queue FIFO when move was reordered before delete.
	list := JobEntriesFromJobs([]*jobs.Job{move, del})
	if len(list) != 2 {
		t.Fatalf("JobsList len = %d, want 2 (delete not removed when move exists)", len(list))
	}
	marked, st := EntryPathJobMarkStatus(child, list)
	if !marked {
		t.Fatal("expected child path marked")
	}
	if st != string(jobs.StatusQueued) {
		t.Fatalf("status = %q, want queued (delete should win for nested path)", st)
	}
}
