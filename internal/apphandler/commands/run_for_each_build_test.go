package commands

import (
	"os/exec"
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
	got, err := BuildRunForEachItem(`echo %f >> /tmp/out`, ent, active, nil, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Argv) != 3 || got.Argv[0] == "" || got.Argv[1] != "-c" {
		t.Fatalf("argv = %#v, want sh -c script", got.Argv)
	}
	if want := `echo '/work/proj/alpha.txt' >> /tmp/out`; got.Argv[2] != want {
		t.Fatalf("script = %q want %q", got.Argv[2], want)
	}
}

func TestBuildRunForEachItemRequiresF(t *testing.T) {
	active := &panel.State{Path: pathloc.MustParse("/work/proj")}
	ent := localfs.Entry{Name: "alpha.txt", Path: "/work/proj/alpha.txt", Type: localfs.EntryFile}
	_, err := BuildRunForEachItem(`gzip -9`, ent, active, nil, false, true)
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
	got, err := BuildRunForEachItem(`gzip -9 %f`, ent, active, nil, false, true)
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
	got, err := BuildRunForEachItem(`echo %f`, ent, active, nil, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Argv) != 2 || got.Argv[0] != "echo" || got.Argv[1] != "/work/proj/beta.log" {
		t.Fatalf("argv = %#v", got.Argv)
	}
}

func TestBuildRunForEachItemNoFRequiredWhenNotRequired(t *testing.T) {
	active := &panel.State{Path: pathloc.MustParse("/work/proj")}
	ent := localfs.Entry{Name: "repo-one", Path: "/work/proj/repo-one", Type: localfs.EntryDirectory}
	got, err := BuildRunForEachItem(`git pull`, ent, active, nil, false, false)
	if err != nil {
		t.Fatalf("unexpected error with requireF=false: %v", err)
	}
	if len(got.Argv) != 2 || got.Argv[0] != "git" || got.Argv[1] != "pull" {
		t.Fatalf("argv = %#v", got.Argv)
	}
}

func TestBuildRunForEachItemHostilePathMacros(t *testing.T) {
	active := &panel.State{Path: pathloc.MustParse("/work/proj")}
	cases := []struct {
		name string
		path string
	}{
		{"command substitution", "/work/proj/beacon$(echo INJECTED)"},
		{"backticks", "/work/proj/lantern`echo INJECTED`"},
		{"dollar home", "/work/proj/meadow$HOME"},
		{"double quotes", `/work/proj/harbor "quoted"`},
		{"single quote", "/work/proj/harbor's-lantern"},
		{"spaces", "/work/proj/orchard meadow.txt"},
		{"embedded newline", "/work/proj/meadow\norchard"},
	}
	for _, tc := range cases {
		t.Run("shell/"+tc.name, func(t *testing.T) {
			ent := localfs.Entry{Name: "harbor", Path: tc.path, Type: localfs.EntryFile}
			got, err := BuildRunForEachItem(`printf '%s\n' %f | cat`, ent, active, nil, false, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Argv) != 3 || got.Argv[1] != "-c" {
				t.Fatalf("argv = %#v, want sh -c", got.Argv)
			}
			out, err := exec.Command(got.Argv[0], got.Argv[1:]...).Output()
			if err != nil {
				t.Fatalf("run: %v\nscript=%q", err, got.Argv[2])
			}
			if gotOut := strings.TrimRight(string(out), "\n"); gotOut != tc.path {
				t.Fatalf("stdout = %q want %q (script %q)", gotOut, tc.path, got.Argv[2])
			}
		})
		t.Run("argv/"+tc.name, func(t *testing.T) {
			ent := localfs.Entry{Name: "harbor", Path: tc.path, Type: localfs.EntryFile}
			got, err := BuildRunForEachItem(`gzip -9 %f`, ent, active, nil, false, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Argv) != 3 || got.Argv[0] != "gzip" || got.Argv[1] != "-9" || got.Argv[2] != tc.path {
				t.Fatalf("argv = %#v", got.Argv)
			}
		})
	}
}
