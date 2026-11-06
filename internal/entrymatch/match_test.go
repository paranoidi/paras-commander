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

// `t t` is true exactly when %t has tagged paths: in a tree layout a tag in an expanded folder
// counts, in the flat layout it does not, and an untagged (false) entry never does.
func TestEvalWhenTaggedMatchesPercentT(t *testing.T) {
	root := t.TempDir()
	orchard := filepath.Join(root, "orchard")
	if err := os.Mkdir(orchard, 0o755); err != nil {
		t.Fatal(err)
	}
	ember := filepath.Join(orchard, "ember.txt")
	if err := os.WriteFile(ember, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	newPanel := func(layout panel.ListLayout) *panel.State {
		ps, err := panel.New(root)
		if err != nil {
			t.Fatal(err)
		}
		if layout == panel.ListLayoutTree {
			if !ps.SetListLayout(panel.ListLayoutTree, 10) {
				t.Fatal("SetListLayout(Tree) = false")
			}
			if err := ps.ToggleTreeExpand(10); err != nil { // cursor starts on orchard
				t.Fatal(err)
			}
		}
		return &ps
	}
	tagged := func(ps *panel.State, paths ...string) *panel.State {
		ps.SelectedPaths = map[string]bool{}
		for _, p := range paths {
			ps.SelectedPaths[p] = true
		}
		return ps
	}
	cases := []struct {
		name string
		ps   *panel.State
		want bool
	}{
		{"tree, only tag in an expanded folder", tagged(newPanel(panel.ListLayoutTree), ember), true},
		{"tree, tag in the panel directory", tagged(newPanel(panel.ListLayoutTree), orchard), true},
		{"flat, tag in the panel directory", tagged(newPanel(panel.ListLayoutFlat), orchard), true},
		{"flat, only tag in a subfolder", tagged(newPanel(panel.ListLayoutFlat), ember), false},
		{"tree, no tags", newPanel(panel.ListLayoutTree), false},
		{"flat, only a ghost tag", tagged(newPanel(panel.ListLayoutFlat), pathloc.FileIndexPrefix+filepath.Join(root, "phantom.txt")), true},
		{"flat, ghost tag in a subfolder", tagged(newPanel(panel.ListLayoutFlat), pathloc.FileIndexPrefix+filepath.Join(orchard, "phantom.txt")), false},
		{"tree, ghost tag in an expanded folder", tagged(newPanel(panel.ListLayoutTree), pathloc.FileIndexPrefix+filepath.Join(orchard, "phantom.txt")), true},
		{"ghost tag of another directory", tagged(newPanel(panel.ListLayoutTree), pathloc.FileIndexPrefix+filepath.Join(filepath.Dir(root), "phantom.txt")), false},
		{"tree, entry switched off", func() *panel.State {
			ps := tagged(newPanel(panel.ListLayoutTree), ember)
			ps.SelectedPaths[ember] = false
			return ps
		}(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EvalWhen("t t", &Context{Active: tc.ps})
			if err != nil || got != tc.want {
				t.Fatalf("t t = %v %v, want %v", got, err, tc.want)
			}
			if has := len(tc.ps.CommandTags()) > 0; has != tc.want {
				t.Fatalf("CommandTags non-empty = %v, want %v", has, tc.want)
			}
		})
	}
}
