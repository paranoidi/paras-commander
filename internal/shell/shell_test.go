package shell

import (
	"os"
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
