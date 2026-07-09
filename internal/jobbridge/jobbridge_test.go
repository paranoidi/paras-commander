package jobbridge

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestEventUpdatesMarks(t *testing.T) {
	t.Parallel()
	if !EventUpdatesMarks(jobs.EventJobResumed) {
		t.Fatal("EventJobResumed should refresh path marks")
	}
	if EventUpdatesMarks(jobs.EventProgress) {
		t.Fatal("EventProgress should not refresh path marks")
	}
}

func TestUniqueParents(t *testing.T) {
	t.Parallel()
	paths, err := pathloc.ParseAll([]string{"/tmp/a/x.txt", "/tmp/a/y.txt", "/tmp/b/z.txt"})
	if err != nil {
		t.Fatal(err)
	}
	got := uniqueParents(paths)
	if len(got) != 2 {
		t.Fatalf("uniqueParents = %d entries, want 2", len(got))
	}
	if got[0].String() != "/tmp/a" || got[1].String() != "/tmp/b" {
		t.Fatalf("uniqueParents = %v, want [/tmp/a /tmp/b]", got)
	}
}
