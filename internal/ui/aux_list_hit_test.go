package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/cmdmacro"
	"github.com/paranoidi/paras-commander/internal/jobs"
)

func auxHitLayout() Layout {
	return Layout{
		Width: 100, Height: 30,
		Menu:      Rect{Width: 100, Height: 1},
		Primary:   Rect{X: 0, Y: 1, Width: 50, Height: 28},
		Secondary: Rect{X: 50, Y: 1, Width: 50, Height: 28},
		Footer:    Rect{Y: 29, Width: 100, Height: 1},
	}
}

func TestJobsListEntryAt(t *testing.T) {
	layout := auxHitLayout()
	model := &Model{JobsList: []JobEntry{{ID: "a"}, {ID: "b"}, {ID: "c", Status: string(jobs.StatusFailed), Error: "disk full"}}}
	// Rows start below the border and the header.
	if idx, ok := JobsListEntryAt(layout, model, 5, 4); !ok || idx != 1 {
		t.Fatalf("row 4 = %d,%v want 1,true", idx, ok)
	}
	for _, p := range [][2]int{{5, 2}, {5, 6}, {0, 3}, {60, 3}} {
		if _, ok := JobsListEntryAt(layout, model, p[0], p[1]); ok {
			t.Errorf("%v hit, want miss (header, past the last row, border, other column)", p)
		}
	}
	model.JobsView = JobsViewState{Selected: 2, Maximized: true, FocusPane: 2} // list, detail, errors
	if _, ok := JobsListEntryAt(layout, model, 5, 3); ok {
		t.Fatal("maximized Errors pane covers the list")
	}
}

func TestCommandsListEntryAtCoversMacroLines(t *testing.T) {
	layout := auxHitLayout()
	model := &Model{CommandsDisplay: []CommandRunEntry{
		{ID: "old", UserCommandLine: "echo walnut"},
		{ID: "new", UserCommandLine: "cp %f", Macros: []cmdmacro.MacroUse{{Letter: 'f', Values: []string{"/srv/pepper"}}}},
	}}
	// Newest first: "new" on rows 2-3 (command + macro), "old" on row 4.
	for y, want := range map[int]int{2: 1, 3: 1, 4: 0} {
		if idx, ok := CommandsListEntryAt(layout, model, 5, y); !ok || idx != want {
			t.Errorf("row %d = %d,%v want %d,true", y, idx, ok, want)
		}
	}
	if _, ok := CommandsListEntryAt(layout, model, 5, 5); ok {
		t.Error("row past the list hit")
	}
}
