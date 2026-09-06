package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestPathEqualOrUnder(t *testing.T) {
	t.Parallel()
	root := filepath.Join("/tmp", "example", "foo")
	if !pathloc.EqualOrUnderStrings(root, root) {
		t.Fatal("root should match itself")
	}
	child := filepath.Join(root, "bar", "asdf")
	if !pathloc.EqualOrUnderStrings(root, child) {
		t.Fatal("child under root")
	}
	if pathloc.EqualOrUnderStrings(root, filepath.Join("/tmp", "example", "other")) {
		t.Fatal("sibling path should not match")
	}
	if pathloc.EqualOrUnderStrings(filepath.Join("/tmp", "example", "foobar"), filepath.Join("/tmp", "example", "foo")) {
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
	if EntryPathMarkedByJobsFromEntries(child, []JobEntry{j}) {
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
	if !EntryPathMarkedByJobsFromEntries(root, []JobEntry{j}) {
		t.Fatal("source root")
	}
	if !EntryPathMarkedByJobsFromEntries(filepath.Join(root, "bar", "asdf"), []JobEntry{j}) {
		t.Fatal("nested under source")
	}
	if EntryPathMarkedByJobsFromEntries(filepath.Join(tmp, "other"), []JobEntry{j}) {
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
	if !EntryPathMarkedByJobsFromEntries(dstRoot, []JobEntry{j}) {
		t.Fatal("resolved dest root")
	}
	if !EntryPathMarkedByJobsFromEntries(filepath.Join(dstRoot, "nested"), []JobEntry{j}) {
		t.Fatal("under resolved dest root")
	}
	if EntryPathMarkedByJobsFromEntries(filepath.Join(dstParent, "unrelated"), []JobEntry{j}) {
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
		Type:    string(jobs.TypeDelete),
		Status:  string(jobs.StatusQueued),
		Sources: []string{child},
	}
	// Same ordering issue as user report: transfer job appears first in JobsList.
	list := []JobEntry{move, del}
	marked, st, _ := EntryPathJobMarkStatusFromEntries(child, list)
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
		Type:    string(jobs.TypeDelete),
		Status:  string(jobs.StatusCompleted),
		Sources: []string{child},
	}
	list := []JobEntry{move, delDone}
	marked, st, _ := EntryPathJobMarkStatusFromEntries(child, list)
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
	marked, st, _ := EntryPathJobMarkStatusFromEntries(deep, list)
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
	marked, st, _ := EntryPathJobMarkStatusFromEntries(row, list)
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
		Sources:     pathloc.PathsForTest(parent),
		Destination: pathloc.MustParse(dst),
		DestIsDir:   true,
	}
	del := &jobs.Job{
		ID:         "job-del",
		Type:       jobs.TypeDelete,
		Status:     jobs.StatusQueued,
		Sources:    pathloc.PathsForTest(child),
		TotalFiles: 1,
	}
	// Same slice order as AllJobs queue FIFO when move was reordered before delete.
	list := JobEntriesFromJobs([]*jobs.Job{move, del}, true, nil)
	if len(list) != 2 {
		t.Fatalf("JobsList len = %d, want 2 (delete not removed when move exists)", len(list))
	}
	marked, st, _ := EntryPathJobMarkStatusFromEntries(child, list)
	if !marked {
		t.Fatal("expected child path marked")
	}
	if st != string(jobs.StatusQueued) {
		t.Fatalf("status = %q, want queued (delete should win for nested path)", st)
	}
}

func TestEntryPathJobMarkStatus_decisionThenRunning(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dstRoot := filepath.Join(tmp, "dest", "pinmonitor")
	if err := os.MkdirAll(dstRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	srcRoot := filepath.Join(tmp, "src", "pinmonitor")
	if err := os.MkdirAll(srcRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	marks := []JobPathMark{{
		Type:        string(jobs.TypeCopy),
		Status:      string(jobs.StatusWaitingDecision),
		Sources:     []string{srcRoot},
		Destination: filepath.Join(tmp, "dest"),
		DestIsDir:   true,
	}}
	marked, st, _ := EntryPathJobMarkStatus(dstRoot, marks)
	if !marked || st != string(jobs.StatusWaitingDecision) {
		t.Fatalf("marked=%v status=%q, want decision on destination dir", marked, st)
	}
	marks[0].Status = string(jobs.StatusRunning)
	marked, st, _ = EntryPathJobMarkStatus(dstRoot, marks)
	if !marked || st != string(jobs.StatusRunning) {
		t.Fatalf("marked=%v status=%q, want running after resume", marked, st)
	}
}

func TestEntryPathJobMarkStatus_roleSourceIsRead(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	src := filepath.Join(tmp, "walnut")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(tmp, "granite")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	marks := []JobPathMark{{
		Type:        string(jobs.TypeCopy),
		Status:      string(jobs.StatusRunning),
		Sources:     []string{src},
		Destination: dst,
		DestIsDir:   true,
	}}
	marked, _, write := EntryPathJobMarkStatus(src, marks)
	if !marked || write {
		t.Fatalf("marked=%v write=%v, want marked read-only source", marked, write)
	}
}

func TestEntryPathJobMarkStatus_roleDestinationIsWrite(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	src := filepath.Join(tmp, "walnut")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(tmp, "granite")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	marks := []JobPathMark{{
		Type:        string(jobs.TypeCopy),
		Status:      string(jobs.StatusRunning),
		Sources:     []string{src},
		Destination: dst,
		DestIsDir:   true,
	}}
	resolvedDst := filepath.Join(dst, filepath.Base(src))
	marked, _, write := EntryPathJobMarkStatus(resolvedDst, marks)
	if !marked || !write {
		t.Fatalf("marked=%v write=%v, want marked write destination", marked, write)
	}
}

func TestEntryPathJobMarkStatus_roleDeleteSourceIsWrite(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	src := filepath.Join(tmp, "walnut")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	marks := []JobPathMark{{
		Type:    string(jobs.TypeDelete),
		Status:  string(jobs.StatusRunning),
		Sources: []string{src},
	}}
	marked, _, write := EntryPathJobMarkStatus(src, marks)
	if !marked || !write {
		t.Fatalf("marked=%v write=%v, want marked write (delete mutates its source)", marked, write)
	}
}

func TestPanelInsideJobWriteTree(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	src := filepath.Join(tmp, "walnut")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(tmp, "granite")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	resolvedRoot := filepath.Join(dst, filepath.Base(src))
	if err := os.MkdirAll(filepath.Join(resolvedRoot, "lantern"), 0o755); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(tmp, "harbor")
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}

	exactDestJob := JobPathMark{
		Type:        string(jobs.TypeCopy),
		Status:      string(jobs.StatusRunning),
		Sources:     []string{src},
		Destination: dst,
		DestIsDir:   true,
	}
	decisionJob := exactDestJob
	decisionJob.Status = string(jobs.StatusWaitingDecision)
	finishedJob := exactDestJob
	finishedJob.Status = string(jobs.StatusCompleted)

	tests := []struct {
		name       string
		panelPath  string
		jobMarks   []JobPathMark
		wantMarked bool
		wantStatus string
	}{
		{
			name:       "exact destination dir match",
			panelPath:  dst,
			jobMarks:   []JobPathMark{exactDestJob},
			wantMarked: true,
			wantStatus: string(jobs.StatusRunning),
		},
		{
			name:       "nested under resolved destination root",
			panelPath:  filepath.Join(resolvedRoot, "lantern"),
			jobMarks:   []JobPathMark{exactDestJob},
			wantMarked: true,
			wantStatus: string(jobs.StatusRunning),
		},
		{
			name:       "sibling directory does not match",
			panelPath:  sibling,
			jobMarks:   []JobPathMark{exactDestJob},
			wantMarked: false,
		},
		{
			name:       "decision status wins over running match",
			panelPath:  dst,
			jobMarks:   []JobPathMark{exactDestJob, decisionJob},
			wantMarked: true,
			wantStatus: string(jobs.StatusWaitingDecision),
		},
		{
			name:       "finished job ignored",
			panelPath:  dst,
			jobMarks:   []JobPathMark{finishedJob},
			wantMarked: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			marked, status := PanelInsideJobWriteTree(tt.panelPath, tt.jobMarks)
			if marked != tt.wantMarked || status != tt.wantStatus {
				t.Fatalf("PanelInsideJobWriteTree(%q) = (%v, %q), want (%v, %q)", tt.panelPath, marked, status, tt.wantMarked, tt.wantStatus)
			}
		})
	}
}

// TestPanelInsideJobWriteTree_destinationContainmentShortCircuit guards the containment
// short-circuit: a panel path that isn't inside the job's destination tree must return false
// without consulting the index, and an exact destination match must still hit — both
// regardless of how many sources the job carries.
func TestPanelInsideJobWriteTree_destinationContainmentShortCircuit(t *testing.T) {
	t.Parallel()
	sources := make([]string, 5000)
	for i := range sources {
		sources[i] = filepath.Join("/src", "file_"+string(rune('a'+i%26)))
	}
	job := JobPathMark{
		Type:        string(jobs.TypeCopy),
		Status:      string(jobs.StatusRunning),
		Sources:     sources,
		Destination: "/dest",
		DestIsDir:   true,
	}
	if marked, _ := PanelInsideJobWriteTree("/unrelated/elsewhere", []JobPathMark{job}); marked {
		t.Fatal("panel path outside the job's destination tree must not be marked")
	}
	if marked, status := PanelInsideJobWriteTree("/dest", []JobPathMark{job}); !marked || status != string(jobs.StatusRunning) {
		t.Fatalf("exact destination dir match should still hit regardless of Sources size; got (%v, %q)", marked, status)
	}
}

// TestEntryPathJobMarkStatus_matchesRowsForVeryLargeSourceLists guards the ancestor index:
// per-row source and destination glyphs must keep working for a job built from a huge
// multi-select, since matching walks the row path's ancestors rather than the source list.
func TestEntryPathJobMarkStatus_matchesRowsForVeryLargeSourceLists(t *testing.T) {
	t.Parallel()
	sources := make([]string, 20000)
	for i := range sources {
		sources[i] = filepath.Join("/src", fmt.Sprintf("file_%05d.txt", i))
	}
	job := JobPathMarksFromEntries([]JobEntry{{
		Type:        string(jobs.TypeCopy),
		Status:      string(jobs.StatusRunning),
		Sources:     sources,
		Destination: "/dest",
		DestIsDir:   true,
	}})
	if marked, _, write := EntryPathJobMarkStatus("/src/file_19999.txt", job); !marked || write {
		t.Fatalf("last source should be marked as a read; got marked=%v write=%v", marked, write)
	}
	if marked, _, write := EntryPathJobMarkStatus("/dest/file_00007.txt", job); !marked || !write {
		t.Fatalf("resolved destination should be marked as a write; got marked=%v write=%v", marked, write)
	}
	if marked, _, _ := EntryPathJobMarkStatus("/src/absent.txt", job); marked {
		t.Fatal("path that is neither a source nor a destination must not be marked")
	}
}
