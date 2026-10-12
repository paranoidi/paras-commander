package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestCommandStderrDisplay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		e    CommandRunEntry
		want string
	}{
		{
			name: "stderr wins",
			e:    CommandRunEntry{Stderr: "oops", ErrorMsg: "launch failed"},
			want: "oops",
		},
		{
			name: "error when streams empty",
			e:    CommandRunEntry{ErrorMsg: "exec format error"},
			want: "exec format error",
		},
		{
			name: "no fallback when stdout has data",
			e:    CommandRunEntry{Stdout: "hi", ErrorMsg: "ignored"},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := CommandStderrDisplay(tt.e); got != tt.want {
				t.Fatalf("CommandStderrDisplay() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCommandPanelLinesWrapsLongOutput(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("word ", 30)
	lines := CommandPanelLines(long, 20, tcell.StyleDefault)
	if len(lines) < 2 {
		t.Fatalf("got %d lines, want wrapped output", len(lines))
	}
	for _, ln := range lines {
		if len(ln) > 20 {
			t.Fatalf("line longer than width: %d cells", len(ln))
		}
	}
}

func TestCommandPanelLinesCountsVisibleWidthOnly(t *testing.T) {
	t.Parallel()
	lines := CommandPanelLines("\x1b[31m"+strings.Repeat("a", 20), 20, tcell.StyleDefault)
	if len(lines) != 1 {
		t.Fatalf("got %d rows, want 1", len(lines))
	}
}
