package privexec

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"testing"
)

func fakeShell(script string) func(context.Context, string, ...string) *exec.Cmd {
	return func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", script)
	}
}

func TestArgv(t *testing.T) {
	got := Argv(true, []string{"rm", "-rf", "--", "/srv/otter"})
	want := []string{"sudo", "-S", "-k", "-p", "", "--", "rm", "-rf", "--", "/srv/otter"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Argv(true) = %q, want %q", got, want)
	}
	got = Argv(false, []string{"mkdir", "--", "/srv/otter"})
	want = []string{"sudo", "-n", "--", "mkdir", "--", "/srv/otter"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Argv(false) = %q, want %q", got, want)
	}
}

func TestRunClassifiesResults(t *testing.T) {
	orig := command
	t.Cleanup(func() { command = orig })

	command = fakeShell("echo 'Sorry, try again.' >&2; exit 1")
	if err := Run(context.Background(), "wrong", "true"); !errors.Is(err, ErrBadPassword) {
		t.Fatalf("bad password: err = %v, want ErrBadPassword", err)
	}

	command = fakeShell("echo 'mv: cannot move' >&2; exit 1")
	err := Run(context.Background(), "pw", "mv")
	if err == nil || errors.Is(err, ErrBadPassword) {
		t.Fatalf("other failure: err = %v, want non-password error", err)
	}

	// Password reaches stdin (fake sudo succeeds only when it reads the expected line).
	command = fakeShell(`read l; [ "$l" = "heron" ]`)
	if err := Run(context.Background(), "heron", "true"); err != nil {
		t.Fatalf("stdin password: %v", err)
	}
}

func TestNoPassword(t *testing.T) {
	orig := command
	t.Cleanup(func() { command = orig })
	command = fakeShell("exit 0")
	if !NoPassword(context.Background()) {
		t.Fatal("NoPassword = false, want true")
	}
	command = fakeShell("echo 'sudo: a password is required' >&2; exit 1")
	if NoPassword(context.Background()) {
		t.Fatal("NoPassword = true, want false")
	}
}
