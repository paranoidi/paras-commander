package commands

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/usermenu"
)

func TestValidateRunForEachCommandRequiresF(t *testing.T) {
	_, msg := validateRunForEachCommand("gzip -9", []localfs.Entry{{Path: "/a/b.txt", Name: "b.txt"}}, nil, nil, false)
	if !strings.Contains(msg, usermenu.ErrRunForEachRequiresF) {
		t.Fatalf("msg = %q", msg)
	}
}

func TestValidateRunForEachCommandShowsPreview(t *testing.T) {
	active := &panel.State{Path: pathloc.MustParse("/work/proj")}
	preview, msg := validateRunForEachCommand("echo %f", []localfs.Entry{{Path: "/a/beacon.txt", Name: "beacon.txt"}}, active, nil, false)
	if msg != "" {
		t.Fatalf("msg = %q, want empty", msg)
	}
	if !strings.Contains(preview, "beacon.txt") {
		t.Fatalf("preview = %q, want it to contain the entry path", preview)
	}
}

func TestValidateRunForEachCommandInDirsRejectsFiles(t *testing.T) {
	active := &panel.State{Path: pathloc.MustParse("/work/proj")}
	entries := []localfs.Entry{
		{Path: "/a/harbor", Name: "harbor", Type: localfs.EntryDirectory},
		{Path: "/a/lantern.txt", Name: "lantern.txt", Type: localfs.EntryFile},
	}
	preview, msg := validateRunForEachCommand("git pull", entries, active, nil, true)
	if preview != "" {
		t.Fatalf("preview = %q, want empty on validation error", preview)
	}
	if !strings.Contains(msg, "directories") {
		t.Fatalf("msg = %q, want directories-only error", msg)
	}
}

func TestValidateRunForEachCommandInDirsAllowsDirsWithoutF(t *testing.T) {
	active := &panel.State{Path: pathloc.MustParse("/work/proj")}
	entries := []localfs.Entry{
		{Path: "/a/harbor", Name: "harbor", Type: localfs.EntryDirectory},
	}
	preview, msg := validateRunForEachCommand("git pull", entries, active, nil, true)
	if msg != "" {
		t.Fatalf("msg = %q, want no error when %%f is omitted in dir mode", msg)
	}
	if preview == "" {
		t.Fatal("expected a preview line")
	}
}

func TestRecordRunForEachHistoryDedupAndCap(t *testing.T) {
	h := &Handler{}

	for range maxRunForEachHistory + 5 {
		h.recordRunForEachHistory("echo %f")
	}
	if got := h.RunForEachHistory(); len(got) != 1 {
		t.Fatalf("history = %+v, want deduped to 1 entry", got)
	}

	for i := range maxRunForEachHistory + 5 {
		h.recordRunForEachHistory("echo " + string(rune('a'+i%26)))
	}
	got := h.RunForEachHistory()
	if len(got) != maxRunForEachHistory {
		t.Fatalf("history len = %d, want cap %d", len(got), maxRunForEachHistory)
	}
	if got[0] != "echo y" {
		t.Fatalf("history[0] = %q, want most-recent %q", got[0], "echo y")
	}
}
