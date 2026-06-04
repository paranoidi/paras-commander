package cmdmacro

import (
	"strings"
	"testing"
)

func TestExpandCommandLineSpecialCharsInName(t *testing.T) {
	got, err := ExpandCommandLine(`gzip -9 %f`, Context{
		Active: &PanelSnapshot{
			Dir:         "/tmp",
			HasCurrent:  true,
			CurrentName: `say "hi".gz`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"`) {
		t.Fatalf("expected quoted expansion, got %q", got)
	}
}

func TestExpandCommandLineRowPath(t *testing.T) {
	got, err := ExpandCommandLine(`wc -l < %f`, Context{RowPath: `/tmp/a b/spaced.txt`})
	if err != nil {
		t.Fatal(err)
	}
	if got != `wc -l < "/tmp/a b/spaced.txt"` {
		t.Fatalf("got %q", got)
	}
}

func TestReplaceTerminalWidth(t *testing.T) {
	got := ReplaceTerminalWidth(`bat --tw=%w --w=%w`, 42)
	want := `bat --tw=42 --w=42`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
