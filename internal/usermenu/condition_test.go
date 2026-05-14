package usermenu

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
)

func TestEvalWhenEmpty(t *testing.T) {
	ctx := &EvalContext{}
	ok, err := EvalWhen("", ctx)
	if err != nil || !ok {
		t.Fatalf("EvalWhen(\"\") = %v %v", ok, err)
	}
}

func TestEvalWhenSimpleGlob(t *testing.T) {
	ps := &panel.State{
		Path: "/tmp",
		Entries: []localfs.Entry{
			{Name: "foo.go", Path: "/tmp/foo.go", Type: localfs.EntryFile},
		},
		Cursor: 0,
	}
	ctx := &EvalContext{Active: ps, Other: ps, ShellPatterns: true}
	ok, err := EvalWhen(`f *.go`, ctx)
	if err != nil || !ok {
		t.Fatalf("f *.go = %v %v", ok, err)
	}
	ok, err = EvalWhen(`f *.txt`, ctx)
	if err != nil || ok {
		t.Fatalf("f *.txt = %v %v", ok, err)
	}
}

func TestEvalWhenNot(t *testing.T) {
	ps := &panel.State{Path: "/x", Entries: []localfs.Entry{{Name: "a", Path: "/x/a", Type: localfs.EntryFile}}, Cursor: 0}
	ctx := &EvalContext{Active: ps, ShellPatterns: true}
	ok, err := EvalWhen(`! f *.txt`, ctx)
	if err != nil || !ok {
		t.Fatalf("! f *.txt = %v %v", ok, err)
	}
}

func TestExpandCommandEchoDir(t *testing.T) {
	ps := &panel.State{Path: "/home/u/proj"}
	got, err := ExpandCommand(`echo %d`, ps, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := `echo "/home/u/proj"`; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
