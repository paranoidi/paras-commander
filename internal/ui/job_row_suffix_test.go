package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestJobRowSuffixIconsByTypeAndStatus(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	mark := func(typ jobs.Type, status jobs.Status) []JobPathMark {
		return []JobPathMark{{Type: string(typ), Status: string(status), Sources: []string{"/src/willow.txt"}, Destination: "/dst", DestIsDir: true}}
	}

	s, st := jobRowSuffix("/src/willow.txt", mark(jobs.TypeMove, jobs.StatusQueued), th)
	if st != string(jobs.StatusQueued) || s.JobIcon != 0 || s.JobQueuedIcon != th.IconFilelistQueued() || s.JobOpIcon != th.IconFilelistMove() || s.JobWrite {
		t.Fatalf("queued move: %+v status=%q", s, st)
	}
	s, _ = jobRowSuffix("/src/willow.txt", mark(jobs.TypeMove, jobs.StatusScanning), th)
	if s.JobIcon != th.IconFilelistJob() || s.JobQueuedIcon != 0 || s.JobOpIcon != th.IconFilelistMove() {
		t.Fatalf("scanning move: %+v", s)
	}
	s, _ = jobRowSuffix("/dst/willow.txt", mark(jobs.TypeMove, jobs.StatusRunning), th)
	if s.JobIcon != th.IconFilelistJob() || !s.JobWrite || s.JobOpIcon != 0 {
		t.Fatalf("running move destination: %+v", s)
	}
	s, _ = jobRowSuffix("/dst/willow.txt", mark(jobs.TypeCopy, jobs.StatusPaused), th)
	if s.JobIcon != 0 || s.JobQueuedIcon != 0 || s.JobOpIcon != 0 || s.JobWrite {
		t.Fatalf("paused copy: %+v", s)
	}
	s, _ = jobRowSuffix("/src/willow.txt", mark(jobs.TypeDelete, jobs.StatusRunning), th)
	if s.JobQueuedIcon != 0 || s.JobOpIcon != th.IconFilelistDelete() || !s.JobWrite {
		t.Fatalf("running delete: %+v", s)
	}
	s, _ = jobRowSuffix("/src/willow.txt", mark(jobs.TypeCopy, jobs.StatusRunning), th)
	if s.JobIcon == 0 || s.JobQueuedIcon != 0 || s.JobOpIcon != 0 {
		t.Fatalf("running copy: %+v", s)
	}
	s, _ = jobRowSuffix("/dst/willow.txt", mark(jobs.TypeCopy, jobs.StatusRunning), th)
	if s.JobIcon != th.IconFilelistJob() || !s.JobWrite || s.JobQueuedIcon != 0 || s.JobOpIcon != 0 {
		t.Fatalf("running copy destination: %+v", s)
	}
	if s, _ := jobRowSuffix("/elsewhere.txt", mark(jobs.TypeMove, jobs.StatusQueued), th); s.JobIcon != 0 {
		t.Fatalf("unrelated path got job suffix: %+v", s)
	}
}

// TestJobRowSuffixFollowsActiveSource: a running multi-source job puts the job icon only on
// the source it works on (both sides), the queued icon on sources still to come (source rows
// only), and nothing on finished ones.
func TestJobRowSuffixFollowsActiveSource(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	build := func(typ jobs.Type, status jobs.Status, active int, phases []jobs.SourcePhase) []JobPathMark {
		return []JobPathMark{withIndex(JobPathMark{
			ID: "harbor", Type: string(typ), Status: string(status),
			Sources:     []string{"/orchard/acorn", "/orchard/bramble", "/orchard/clover"},
			Destination: "/meadow", DestIsDir: true,
			SourcePhases: phases, ActiveSource: active,
		})}
	}
	midway := []jobs.SourcePhase{jobs.SourceFinished, jobs.SourceStarted, jobs.SourceQueued}
	copyMarks := build(jobs.TypeCopy, jobs.StatusRunning, 1, midway)

	cases := []struct {
		path               string
		job, queued, write bool
	}{
		{"/orchard/bramble/deep/thistle.txt", true, false, false},
		{"/meadow/bramble/deep/thistle.txt", true, false, true},
		{"/orchard/clover", false, true, false},
		{"/meadow/clover", false, false, false},
		{"/orchard/acorn/sorrel.txt", false, false, false},
		{"/meadow/acorn", false, false, false},
	}
	for _, c := range cases {
		s, _ := jobRowSuffix(c.path, copyMarks, th)
		if (s.JobIcon != 0) != c.job || (s.JobQueuedIcon != 0) != c.queued || s.JobWrite != c.write {
			t.Errorf("%s: %+v, want job=%v queued=%v write=%v", c.path, s, c.job, c.queued, c.write)
		}
	}
	if _, st := jobRowSuffix("/orchard/clover", copyMarks, th); st != string(jobs.StatusQueued) {
		t.Fatalf("pending source status = %q, want queued", st)
	}

	moveMarks := build(jobs.TypeMove, jobs.StatusRunning, 1, midway)
	if s, _ := jobRowSuffix("/orchard/clover", moveMarks, th); s.JobQueuedIcon == 0 || s.JobOpIcon != th.IconFilelistMove() || s.JobIcon != 0 {
		t.Fatalf("pending move source: %+v", s)
	}

	// Before the first source starts, and for a queued job, every root marks as before.
	for _, m := range [][]JobPathMark{
		build(jobs.TypeCopy, jobs.StatusRunning, -1, make([]jobs.SourcePhase, 3)),
		build(jobs.TypeCopy, jobs.StatusQueued, 1, midway),
	} {
		active, _ := jobRowSuffix("/meadow/acorn", m, th)
		queued := jobs.Status(m[0].Status) == jobs.StatusQueued
		if !queued && active.JobIcon == 0 || queued && active.JobQueuedIcon == 0 {
			t.Fatalf("%s active=%d: finished-source destination lost its mark: %+v", m[0].Status, m[0].ActiveSource, active)
		}
	}
}

// TestJobRowSuffixMoveRemovingSources: once a move's copy fallback deletes its sources, every
// source row (finished ones included) shows the job icon and the removing icon, and no
// destination row is marked.
func TestJobRowSuffixMoveRemovingSources(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	marks := []JobPathMark{withIndex(JobPathMark{
		ID: "harbor", Type: string(jobs.TypeMove), Status: string(jobs.StatusRunning),
		Sources:     []string{"/orchard/acorn", "/orchard/bramble", "/orchard/clover"},
		Destination: "/meadow", DestIsDir: true,
		SourcePhases:    []jobs.SourcePhase{jobs.SourceFinished, jobs.SourceFinished, jobs.SourceStarted},
		ActiveSource:    2,
		RemovingSources: true,
	})}
	for _, p := range []string{"/orchard/acorn", "/orchard/bramble/deep/thistle.txt", "/orchard/clover"} {
		s, _ := jobRowSuffix(p, marks, th)
		if s.JobIcon != th.IconFilelistJob() || s.JobOpIcon != th.IconFilelistMoveRemoving() || s.JobOpStyle != th.PanelRowMarkJobRemoving || s.JobWrite {
			t.Errorf("%s: %+v, want job + removing icons", p, s)
		}
	}
	for _, p := range []string{"/meadow/acorn", "/meadow/clover"} {
		if s, _ := jobRowSuffix(p, marks, th); s != (panellist.JobSuffix{}) {
			t.Errorf("%s: destination marked while removing sources: %+v", p, s)
		}
	}
	if hit, _ := PanelInsideJobWriteTree("/meadow/clover", marks); hit {
		t.Error("destination tree still reported as written while removing sources")
	}

	marks[0].RemovingSources = false
	if s, _ := jobRowSuffix("/orchard/clover", marks, th); s.JobOpIcon != th.IconFilelistMove() {
		t.Fatalf("move source before removal: %+v", s)
	}
}
