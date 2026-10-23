package jobs

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestUndoableTruthTable(t *testing.T) {
	steps := []ops.UndoStep{{Kind: ops.UndoRemoveCreated}}
	cases := []struct {
		name string
		job  Job
		want bool
	}{
		{"completed copy", Job{Type: TypeCopy, Status: StatusCompleted, Undo: steps}, true},
		{"canceled copy keeps partial journal", Job{Type: TypeCopy, Status: StatusCanceled, Undo: steps}, true},
		{"failed move", Job{Type: TypeMove, Status: StatusFailed, Undo: steps}, true},
		{"rename", Job{Type: TypeRename, Status: StatusCompleted, Undo: steps}, true},
		{"running", Job{Type: TypeCopy, Status: StatusRunning, Undo: steps}, false},
		{"empty journal", Job{Type: TypeCopy, Status: StatusCompleted}, false},
		{"irreversible", Job{Type: TypeCopy, Status: StatusCompleted, Undo: steps, Irreversible: true}, false},
		{"already undone", Job{Type: TypeCopy, Status: StatusCompleted, Undo: steps, Undone: true}, false},
		{"delete", Job{Type: TypeDelete, Status: StatusCompleted, Undo: steps}, false},
		{"undo job itself", Job{Type: TypeUndo, Status: StatusCompleted, Undo: steps}, false},
	}
	for _, c := range cases {
		if got := c.job.Undoable(); got != c.want {
			t.Errorf("%s: Undoable() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestAddFinishedJobAppearsInAllJobs(t *testing.T) {
	s := NewState()
	s.AddFinishedJob(&Job{ID: "job-rename", Type: TypeRename, Undo: []ops.UndoStep{{Kind: ops.UndoMoveBack}}})
	all := s.AllJobs()
	if len(all) != 1 || all[0].ID != "job-rename" || all[0].Status != StatusCompleted || !all[0].Undoable() {
		t.Fatalf("AllJobs() = %+v", all)
	}
}

func TestJournalEventAndEnqueueUndo(t *testing.T) {
	s := NewState()
	src := pathloc.FileMust("/tmp/meadow-lantern")
	s.AddFinishedJob(&Job{ID: "job-copy", Type: TypeCopy, Sources: []pathloc.Path{src}})
	if _, ok := s.EnqueueUndo("job-copy"); ok {
		t.Fatal("job without a journal must not be undoable")
	}
	s.ApplyEvent(Event{Type: EventUndoJournal, JobID: "job-copy", Undo: []ops.UndoStep{{Kind: ops.UndoRemoveCreated}}})
	undo, ok := s.EnqueueUndo("job-copy")
	if !ok || undo.Type != TypeUndo || undo.UndoOf != "job-copy" || len(undo.Undo) != 1 {
		t.Fatalf("EnqueueUndo = %+v, %v", undo, ok)
	}
	if _, ok := s.EnqueueUndo("job-copy"); ok {
		t.Fatal("a second undo must not queue while the first is unfinished")
	}
	s.ApplyEvent(Event{Type: EventStarted, JobID: undo.ID})
	s.ApplyEvent(Event{Type: EventCompleted, JobID: undo.ID, DoneFiles: 1})
	for _, j := range s.AllJobs() {
		if j.ID == "job-copy" && !j.Undone {
			t.Fatal("target should be marked undone")
		}
	}
	if s.RetryJob(undo.ID) {
		t.Fatal("undo jobs are not retryable")
	}
}
