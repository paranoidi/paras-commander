package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestTopLevelDestNamesFromJobCopyDirectoryOnlyMarksTopLevel(t *testing.T) {
	t.Parallel()
	destDir := t.TempDir()
	srcTree := t.TempDir()
	job := &jobs.Job{
		Type:        jobs.TypeCopy,
		Status:      jobs.StatusCompleted,
		Sources:     pathloc.PathsForTest(srcTree),
		Destination: pathloc.MustParse(destDir),
		DestIsDir:   true,
	}
	dir, names, ok := TopLevelDestNamesFromJob(job)
	if !ok {
		t.Fatal("TopLevelDestNamesFromJob ok = false")
	}
	if dir.String() != destDir {
		t.Fatalf("dest dir = %q, want %q", dir.String(), destDir)
	}
	if len(names) != 1 {
		t.Fatalf("names = %v, want one top-level entry", names)
	}
}

func TestTopLevelDestNamesFromJobSkipsFailedTypes(t *testing.T) {
	t.Parallel()
	job := &jobs.Job{
		Type:        jobs.TypeDelete,
		Status:      jobs.StatusCompleted,
		Sources:     pathloc.PathsForTest("/tmp/a"),
		Destination: pathloc.MustParse("/tmp/b"),
		DestIsDir:   true,
	}
	if _, _, ok := TopLevelDestNamesFromJob(job); ok {
		t.Fatal("delete job should not produce new-file marks")
	}
}
