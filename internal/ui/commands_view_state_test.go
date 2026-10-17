package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/cmdmacro"
)

func TestCommandsEnsureSelectionVisibleTwoLineRows(t *testing.T) {
	t.Parallel()
	// Rows 0..5; even rows carry one macro (2 lines), odd rows none (1 line).
	entries := make([]CommandRunEntry, 6)
	for i := range entries {
		if i%2 == 0 {
			entries[i].Macros = []cmdmacro.MacroUse{{Letter: 'f', Values: []string{"/lantern/orchard"}}}
		}
	}
	tests := []struct {
		name                 string
		selected, listScroll int
		wantScroll           int
	}{
		{name: "fits first page", selected: 2, listScroll: 0, wantScroll: 0}, // 2+1+2 = 5 lines
		{name: "scrolls by line budget", selected: 3, listScroll: 0, wantScroll: 1},
		{name: "last row clamps to max", selected: 5, listScroll: 0, wantScroll: 3}, // 1+2+1 = 4 ≤ 5, +2 > 5
		{name: "selection above viewport", selected: 1, listScroll: 4, wantScroll: 1},
	}
	for _, tt := range tests {
		s := CommandsViewState{Selected: tt.selected, ListScroll: tt.listScroll}
		s.EnsureSelectionVisible(entries, 5)
		if s.ListScroll != tt.wantScroll {
			t.Errorf("%s: ListScroll=%d, want %d", tt.name, s.ListScroll, tt.wantScroll)
		}
	}
}

func TestCommandDisplayLine(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"bash -ci 'ls -l /'": "'ls -l /'",
		"bash script.sh":     "script.sh",
		"bashful --quiet":    "bashful --quiet",
		"fish -c 'echo hi'":  "'echo hi'",
		"fish -l":            "fish -l",
		"du -sh %f":          "du -sh %f",
	} {
		if got := commandDisplayLine(in); got != want {
			t.Errorf("commandDisplayLine(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCommandMacroLine(t *testing.T) {
	t.Parallel()
	one := cmdmacro.MacroUse{Letter: 'd', Values: []string{"/home/quill/harbor/lantern"}}
	if got := commandMacroLine(one, "/home/quill", 40); got != "%d ~/harbor/lantern" {
		t.Errorf("single: %q", got)
	}
	many := cmdmacro.MacroUse{Letter: 't', Values: []string{"/meadow/a.txt", "/meadow/b.txt", "/meadow/c.txt"}}
	if got := commandMacroLine(many, "", 40); got != "%t /meadow/a.txt (+2 more)" {
		t.Errorf("multi: %q", got)
	}
}
