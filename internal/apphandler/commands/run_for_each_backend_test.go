package commands

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/textutil"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestRunForEachUnifiedBatchPerEntryWorkDir(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()

	dirA := t.TempDir()
	dirB := t.TempDir()
	entries := []localfs.Entry{
		{Name: "dirA", Path: dirA, Type: localfs.EntryDirectory},
		{Name: "dirB", Path: dirB, Type: localfs.EntryDirectory},
	}

	h := &Handler{
		screen: screen,
		model:  &ui.Model{CommandsList: make([]ui.CommandRunEntry, len(entries))},
		mu:     &sync.RWMutex{},
		ctx:    context.Background(),
	}

	spec := RunForEachBatchSpec{
		Entries:         entries,
		AllowDirs:       true,
		WorkDir:         "/should-not-be-used",
		PerEntryWorkDir: true,
		BuildItem: func(localfs.Entry) (RunForEachBuiltItem, error) {
			return RunForEachBuiltItem{Argv: []string{"pwd"}, UserLine: "pwd"}, nil
		},
	}
	h.runForEachUnifiedBatch(context.Background(), 0, spec)

	for i, ent := range entries {
		got := strings.TrimSpace(h.model.CommandsList[i].Stdout)
		want := textutil.AbsPathClean(ent.Path)
		if got != want {
			t.Fatalf("entry %d: cwd = %q, want %q (err=%q)", i, got, want, h.model.CommandsList[i].ErrorMsg)
		}
	}
}

func TestSummarizeRunForEachIssuesFailed(t *testing.T) {
	entries := []ui.CommandRunEntry{
		{ExitCode: 1, ErrorMsg: "boom"},
		{ExitCode: 0},
	}
	log, banner, urg, ok := summarizeRunForEachIssues("Run for each", entries)
	if !ok {
		t.Fatal("expected issue summary")
	}
	if urg != ui.MessageUrgencyError {
		t.Fatalf("urgency = %v", urg)
	}
	if log != "Run for each: 1 failed (boom)" {
		t.Fatalf("log = %q", log)
	}
	if banner == "" {
		t.Fatal("expected banner")
	}
}

func TestSummarizeRunForEachIssuesOK(t *testing.T) {
	_, _, _, ok := summarizeRunForEachIssues("Run for each", []ui.CommandRunEntry{{ExitCode: 0}})
	if ok {
		t.Fatal("expected no summary for all-success batch")
	}
}
