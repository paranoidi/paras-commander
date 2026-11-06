package entrymatch

import (
	"os"
	"path/filepath"
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

func TestValidateWhenExprs(t *testing.T) {
	if err := ValidateWhenExprs([]string{`*.go`}, true); err != nil {
		t.Fatalf("valid glob: %v", err)
	}
	if err := ValidateWhenExprs([]string{`f [`}, true); err == nil {
		t.Fatal("expected invalid glob error")
	}
	if err := ValidateWhenExprs([]string{`f (`}, false); err == nil {
		t.Fatal("expected syntax error")
	}
	if err := ValidateWhenExprs([]string{`f [`}, false); err == nil {
		t.Fatal("expected invalid regex error")
	}
}

func TestEvalWhenDirUsesCursorDirInTree(t *testing.T) {
	root := t.TempDir()
	orchard := filepath.Join(root, "orchard")
	if err := os.Mkdir(orchard, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orchard, "ember.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	newTree := func() *panel.State {
		ps, err := panel.New(root)
		if err != nil {
			t.Fatal(err)
		}
		if !ps.SetListLayout(panel.ListLayoutTree, 10) {
			t.Fatal("SetListLayout(Tree) = false")
		}
		if err := ps.ToggleTreeExpand(10); err != nil { // cursor starts on orchard
			t.Fatal(err)
		}
		return &ps
	}
	active, other := newTree(), newTree()
	for i := 0; i < active.VisibleEntryCount(); i++ {
		if e, _, ok := active.VisibleEntry(i); ok && e.Name == "ember.txt" {
			active.Cursor = i
			other.Cursor = i
		}
	}
	if got := active.CursorDir().String(); got != orchard {
		t.Fatalf("CursorDir = %q, want %q", got, orchard)
	}
	ctx := &Context{Active: active, Other: other, ShellPatterns: true}
	for _, expr := range []string{"d " + orchard, "D " + orchard} {
		if ok, err := EvalWhen(expr, ctx); err != nil || !ok {
			t.Fatalf("%q = %v %v, want true", expr, ok, err)
		}
	}
	for _, expr := range []string{"d " + root, "D " + root} {
		if ok, err := EvalWhen(expr, ctx); err != nil || ok {
			t.Fatalf("%q = %v %v, want false", expr, ok, err)
		}
	}
	// Empty panel falls back to the panel path.
	empty := &panel.State{Path: pathloc.MustParse(root)}
	if ok, err := EvalWhen("d "+root, &Context{Active: empty, ShellPatterns: true}); err != nil || !ok {
		t.Fatalf("empty panel d = %v %v", ok, err)
	}
}
