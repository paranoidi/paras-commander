package jobbridge

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
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
