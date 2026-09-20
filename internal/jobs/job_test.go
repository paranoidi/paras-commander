package jobs

import (
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"testing"
	"time"
)

func TestJobLifecycle(t *testing.T) {
	job := &Job{
		ID:          NewJobID(),
		Type:        TypeCopy,
		Status:      StatusQueued,
		Sources:     pathloc.PathsForTest("/src/a.txt"),
		Destination: pathloc.MustParse("/dst"),
		TotalFiles:  1,
	}

	if job.Status != StatusQueued {
		t.Fatalf("initial status = %q, want %q", job.Status, StatusQueued)
	}

	job.Status = StatusRunning
	if job.Status != StatusRunning {
		t.Fatalf("running status = %q, want %q", job.Status, StatusRunning)
	}

	job.DoneFiles = 1
	job.DoneBytes = 1024
	job.CurrentPath = "/src/a.txt"
	job.StartedAt = time.Now()
	job.Status = StatusCompleted
	job.FinishedAt = time.Now()

	if !job.Status.IsFinished() {
		t.Fatal("completed should be finished")
	}
}

func TestFinishedStatuses(t *testing.T) {
	for _, s := range FinishedStatuses() {
		if !s.IsFinished() {
			t.Fatalf("%q should be finished", s)
		}
	}
}

func TestNewJobIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for range 100 {
		id := NewJobID()
		if ids[id] {
			t.Fatalf("duplicate job ID: %s", id)
		}
		ids[id] = true
	}
}

func TestHoldsTransferLeaseOverlap(t *testing.T) {
	t.Parallel()
	copyJob := &Job{
		Type:        TypeCopy,
		Sources:     pathloc.PathsForTest("/willow/branch"),
		Destination: pathloc.MustParse("/maple/trunk"),
	}
	moveJob := &Job{
		Type:        TypeMove,
		Sources:     pathloc.PathsForTest("/cedar"),
		Destination: pathloc.MustParse("/oak"),
	}
	extractJob := &Job{
		Type:        TypeExtract,
		Sources:     pathloc.PathsForTest("/archive/bundle.tar"),
		Destination: pathloc.MustParse("/extract/out"),
	}

	cases := []struct {
		name  string
		job   *Job
		other *Job
		want  bool
	}{
		{name: "copy always holds", job: copyJob, other: nil, want: true},
		{name: "move always holds", job: moveJob, other: copyJob, want: true},
		{name: "extract always holds", job: extractJob, other: nil, want: true},
		{
			name: "delete without active transfer skips",
			job:  &Job{Type: TypeDelete, Sources: pathloc.PathsForTest("/willow/branch")},
			want: false,
		},
		{
			name:  "disjoint delete skips",
			job:   &Job{Type: TypeDelete, Sources: pathloc.PathsForTest("/birch")},
			other: copyJob,
			want:  false,
		},
		{
			name:  "delete overlapping transfer source holds",
			job:   &Job{Type: TypeDelete, Sources: pathloc.PathsForTest("/willow/branch")},
			other: copyJob,
			want:  true,
		},
		{
			name:  "delete overlapping transfer dest holds",
			job:   &Job{Type: TypeDelete, Sources: pathloc.PathsForTest("/maple/trunk")},
			other: copyJob,
			want:  true,
		},
		{
			name:  "delete descendant of source holds",
			job:   &Job{Type: TypeDelete, Sources: pathloc.PathsForTest("/willow/branch/leaf.txt")},
			other: copyJob,
			want:  true,
		},
		{
			name:  "delete ancestor of source holds",
			job:   &Job{Type: TypeDelete, Sources: pathloc.PathsForTest("/willow")},
			other: copyJob,
			want:  true,
		},
		{
			name:  "delete descendant of dest holds",
			job:   &Job{Type: TypeDelete, Sources: pathloc.PathsForTest("/maple/trunk/sap.txt")},
			other: copyJob,
			want:  true,
		},
		{
			name:  "delete ancestor of dest holds",
			job:   &Job{Type: TypeDelete, Sources: pathloc.PathsForTest("/maple")},
			other: copyJob,
			want:  true,
		},
		{
			name:  "sibling path does not hold",
			job:   &Job{Type: TypeDelete, Sources: pathloc.PathsForTest("/willow/other")},
			other: copyJob,
			want:  false,
		},
		{
			name: "sftp delete does not overlap local transfer",
			job: &Job{
				Type:    TypeDelete,
				Sources: []pathloc.Path{pathloc.MustParse("sftp://host/willow/branch")},
			},
			other: copyJob,
			want:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.job.holdsTransferLease(tc.other); got != tc.want {
				t.Fatalf("holdsTransferLease() = %v, want %v", got, tc.want)
			}
		})
	}
}
