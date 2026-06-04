package entrymatch

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestEvalWhenBareGlob(t *testing.T) {
	ps := &panel.State{
		Path: pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{
			{Name: "foo.go", Path: "/tmp/foo.go", Type: localfs.EntryFile},
		},
		Cursor: 0,
	}
	ctx := &Context{Active: ps, ShellPatterns: true}
	ok, err := EvalWhen(`*.go`, ctx)
	if err != nil || !ok {
		t.Fatalf("*.go = %v %v", ok, err)
	}
	ok, err = EvalWhen(`*.txt`, ctx)
	if err != nil || ok {
		t.Fatalf("*.txt = %v %v", ok, err)
	}
}

func TestEvalWhenRowBareGlob(t *testing.T) {
	ent := localfs.Entry{Name: "report.py", Path: "/proj/report.py", Type: localfs.EntryFile}
	ctx := &Context{
		ShellPatterns: true,
		Row:           &ent,
		PanelDir:      "/proj",
	}
	ok, err := EvalWhen(`*.py`, ctx)
	if err != nil || !ok {
		t.Fatalf("row *.py = %v %v", ok, err)
	}
}
