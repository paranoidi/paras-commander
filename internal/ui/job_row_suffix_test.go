package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestJobRowSuffixIconsByTypeAndStatus(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	mark := func(typ jobs.Type, status jobs.Status) []JobPathMark {
		return []JobPathMark{{Type: string(typ), Status: string(status), Sources: []string{"/src/willow.txt"}, Destination: "/dst", DestIsDir: true}}
	}

	s, st := jobRowSuffix("/src/willow.txt", mark(jobs.TypeMove, jobs.StatusQueued), th)
	if st != string(jobs.StatusQueued) || s.JobIcon != th.IconFilelistJob() || s.JobQueuedIcon != th.IconFilelistQueued() || s.JobOpIcon != th.IconFilelistMove() || s.JobWrite {
		t.Fatalf("queued move: %+v status=%q", s, st)
	}
	s, _ = jobRowSuffix("/src/willow.txt", mark(jobs.TypeDelete, jobs.StatusRunning), th)
	if s.JobQueuedIcon != 0 || s.JobOpIcon != th.IconFilelistDelete() || !s.JobWrite {
		t.Fatalf("running delete: %+v", s)
	}
	s, _ = jobRowSuffix("/src/willow.txt", mark(jobs.TypeCopy, jobs.StatusRunning), th)
	if s.JobIcon == 0 || s.JobQueuedIcon != 0 || s.JobOpIcon != 0 {
		t.Fatalf("running copy: %+v", s)
	}
	if s, _ := jobRowSuffix("/elsewhere.txt", mark(jobs.TypeMove, jobs.StatusQueued), th); s.JobIcon != 0 {
		t.Fatalf("unrelated path got job suffix: %+v", s)
	}
}
