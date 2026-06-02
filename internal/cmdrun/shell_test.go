package cmdrun

import "testing"

func TestArgvNeedsShellOperatorTokens(t *testing.T) {
	if !ArgvNeedsShell([]string{"echo", "x", ">>", "/tmp/out"}) {
		t.Fatal("expected >> to need shell")
	}
}

func TestNeedsShellFromLineEchoRedirect(t *testing.T) {
	if !NeedsShellFromLine(`echo "a" >> /tmp/out`) {
		t.Fatal("expected shell for >> redirection")
	}
}

func TestNeedsShellFromLineSimpleArgv(t *testing.T) {
	if NeedsShellFromLine(`gzip -9`) {
		t.Fatal("gzip -9 should not need shell")
	}
}

func TestShellArgv(t *testing.T) {
	got := ShellArgv(`echo hi`)
	if len(got) != 3 || got[1] != "-c" || got[2] != `echo hi` {
		t.Fatalf("ShellArgv() = %#v", got)
	}
}
