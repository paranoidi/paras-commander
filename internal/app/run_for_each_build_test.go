package app

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/usermenu"
)

func TestBuildRunForEachItemExpandsFAndUsesShell(t *testing.T) {
	active := &panel.State{Path: pathloc.MustParse("/work/proj")}
	ent := localfs.Entry{Name: "alpha.txt", Path: "/work/proj/alpha.txt", Type: localfs.EntryFile}
	got, err := buildRunForEachItem(`echo %f >> /tmp/out`, ent, active, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Argv) != 3 || got.Argv[0] == "" || got.Argv[1] != "-c" {
		t.Fatalf("argv = %#v, want sh -c script", got.Argv)
	}
	if want := `echo "/work/proj/alpha.txt" >> /tmp/out`; got.Argv[2] != want {
		t.Fatalf("script = %q want %q", got.Argv[2], want)
	}
}

func TestBuildRunForEachItemRequiresF(t *testing.T) {
	active := &panel.State{Path: pathloc.MustParse("/work/proj")}
	ent := localfs.Entry{Name: "alpha.txt", Path: "/work/proj/alpha.txt", Type: localfs.EntryFile}
	_, err := buildRunForEachItem(`gzip -9`, ent, active, nil, false)
	if err == nil {
		t.Fatal("expected error when iterated macro is missing")
	}
	if !strings.Contains(err.Error(), usermenu.ErrRunForEachRequiresF) {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildRunForEachItemPlainArgvWithF(t *testing.T) {
	active := &panel.State{Path: pathloc.MustParse("/work/proj")}
	ent := localfs.Entry{Name: "alpha.txt", Path: "/work/proj/alpha.txt", Type: localfs.EntryFile}
	got, err := buildRunForEachItem(`gzip -9 %f`, ent, active, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Argv) != 3 || got.Argv[0] != "gzip" || got.Argv[1] != "-9" || got.Argv[2] != "/work/proj/alpha.txt" {
		t.Fatalf("argv = %#v", got.Argv)
	}
}

func TestBuildRunForEachItemMacroInArgv(t *testing.T) {
	active := &panel.State{Path: pathloc.MustParse("/work/proj")}
	ent := localfs.Entry{Name: "beta.log", Path: "/work/proj/beta.log", Type: localfs.EntryFile}
	got, err := buildRunForEachItem(`echo %f`, ent, active, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Argv) != 2 || got.Argv[0] != "echo" || got.Argv[1] != "/work/proj/beta.log" {
		t.Fatalf("argv = %#v", got.Argv)
	}
}
