package shell

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveShellUsesEnv(t *testing.T) {
	const want = "/tmp/custom-shell-binary"
	t.Setenv("SHELL", want)
	if got := ResolveShell(); got != want {
		t.Fatalf("ResolveShell() = %q, want %q", got, want)
	}
}

func TestResolveShellFallbackWithoutEnv(t *testing.T) {
	t.Setenv("SHELL", "")
	got := ResolveShell()
	if got == "" {
		t.Fatal("ResolveShell() empty")
	}
	if _, err := os.Stat(got); err != nil && got != defaultShell {
		t.Fatalf("ResolveShell() = %q, not executable and not default", got)
	}
}

func TestRunInteractiveChildCdDoesNotChangeParentCwd(t *testing.T) {
	parent, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	if err := RunInteractive(context.Background(), []string{"sh", "-c", "cd -- " + other}); err != nil {
		t.Fatal(err)
	}
	got, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(parent) {
		t.Fatalf("parent cwd = %q, want %q", got, parent)
	}
}

func TestShellArgv(t *testing.T) {
	argv := ShellArgv("/bin/zsh")
	if len(argv) != 1 || argv[0] != "/bin/zsh" {
		t.Fatalf("ShellArgv() = %v, want [/bin/zsh]", argv)
	}
	empty := ShellArgv("")
	if len(empty) != 1 || empty[0] == "" {
		t.Fatalf("ShellArgv(\"\") = %v, want single non-empty shell", empty)
	}
}
