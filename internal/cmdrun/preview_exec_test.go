package cmdrun

import (
	"reflect"
	"testing"
)

func TestFinalizePreviewArgvWithExecutableBatToBatcat(t *testing.T) {
	argv := []string{"bat", "--paging=never", "--color=always", "/tmp/x.go"}
	got := FinalizePreviewArgvWithExecutable(argv, "/tmp/x.go", "/usr/bin/batcat")
	want := []string{"/usr/bin/batcat", "--paging=never", "--color=always", "/tmp/x.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFinalizePreviewArgvWithExecutableBatToCat(t *testing.T) {
	argv := []string{"bat", "--paging=never", "--terminal-width=80", "/tmp/x.go"}
	got := FinalizePreviewArgvWithExecutable(argv, "/tmp/x.go", "/bin/cat")
	want := []string{"/bin/cat", "/tmp/x.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFinalizePreviewArgvWithExecutableBatcatToCat(t *testing.T) {
	argv := []string{"batcat", "--wrap=auto", "/tmp/a b.txt"}
	got := FinalizePreviewArgvWithExecutable(argv, "/tmp/a b.txt", "/bin/cat")
	want := []string{"/bin/cat", "/tmp/a b.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFinalizePreviewArgvWithExecutableLeavesCustomCommand(t *testing.T) {
	argv := []string{"less", "-N", "/tmp/x.go"}
	got := FinalizePreviewArgvWithExecutable(argv, "/tmp/x.go", "/bin/cat")
	if !reflect.DeepEqual(got, argv) {
		t.Fatalf("got %q want unchanged %q", got, argv)
	}
}

func TestFinalizePreviewArgvWithExecutableEmptyArgv(t *testing.T) {
	got := FinalizePreviewArgvWithExecutable(nil, "/tmp/x.go", "/bin/cat")
	want := []string{"/bin/cat", "/tmp/x.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildFilePreviewArgvIntegratesTemplate(t *testing.T) {
	exe, err := ResolveFilePreviewExecutable()
	if err != nil {
		t.Skipf("no preview executable on PATH: %v", err)
	}
	got, err := BuildFilePreviewArgv(
		"bat --paging=never --terminal-width={terminal_width}",
		"/tmp/x.go",
		42,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 {
		t.Fatalf("argv too short: %q", got)
	}
	if got[0] != exe {
		t.Fatalf("argv[0] = %q want resolved %q", got[0], exe)
	}
	if got[len(got)-1] != "/tmp/x.go" {
		t.Fatalf("last arg = %q want path", got[len(got)-1])
	}
}
