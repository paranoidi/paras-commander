package ops

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestRootFirstErrorText(t *testing.T) {
	path := "/quixotic/marmalade"
	err := fmt.Errorf("move copy phase: %w", fmt.Errorf("create directory %q: %w", path, &os.PathError{Op: "mkdir", Path: path, Err: syscall.EACCES}))
	got := RootFirstErrorText(err)
	if !strings.HasPrefix(got, "permission denied: ") {
		t.Fatalf("RootFirstErrorText(%v) = %q, want prefix %q", err, got, "permission denied: ")
	}
	if !strings.Contains(got, "move copy phase") {
		t.Fatalf("RootFirstErrorText(%v) = %q, want to retain outer context", err, got)
	}
}

func TestRootFirstErrorTextPlainError(t *testing.T) {
	err := fmt.Errorf("scan function not configured")
	if got := RootFirstErrorText(err); got != err.Error() {
		t.Fatalf("RootFirstErrorText(%v) = %q, want %q", err, got, err.Error())
	}
}
