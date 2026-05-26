package jobbridge

import (
	"errors"
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
)

func TestActivityFailureLabel(t *testing.T) {
	t.Parallel()
	err := &ops.Error{Op: "delete", Text: "failed to delete v1", Err: errors.New("directory not empty")}
	ev := jobs.Event{Type: jobs.EventFailed, Error: err.Error(), Err: err}
	got := ActivityFailureLabel(ev)
	want := "Failed: delete: failed to delete v1 (directory not empty)"
	if got != want {
		t.Fatalf("ActivityFailureLabel() = %q, want %q", got, want)
	}
}
