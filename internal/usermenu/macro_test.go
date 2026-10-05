package usermenu

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/cmdmacro"
	"github.com/paranoidi/paras-commander/internal/panel"
)

func TestCommandRequiresIteratedF(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{`gzip -9 %f`, true},
		{`gzip -9`, false},
		{`echo %%f`, false},
		{`echo %f %d`, true},
	}
	for _, tc := range tests {
		if got := CommandRequiresIteratedF(tc.cmd); got != tc.want {
			t.Errorf("CommandRequiresIteratedF(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

func TestExpandCommandTreeDirFollowsCaretRow(t *testing.T) {
	root := t.TempDir()
	orchard := filepath.Join(root, "orchard")
	if err := os.Mkdir(orchard, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orchard, "lantern.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := panel.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := ExpandCommand("%d", &state, nil); got != cmdmacro.QuoteShellArg(root) {
		t.Fatalf("flat %%d = %s, want %s", got, root)
	}
	if !state.SetListLayout(panel.ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false")
	}
	if err := state.ToggleTreeExpand(10); err != nil { // cursor on "orchard"
		t.Fatal(err)
	}
	if got, _ := ExpandCommand("%d", &state, nil); got != cmdmacro.QuoteShellArg(root) {
		t.Fatalf("tree %%d on dir row = %s, want %s", got, root)
	}
	state.Cursor = 1 // orchard/lantern.txt
	if got, _ := ExpandCommand("%d", &state, nil); got != cmdmacro.QuoteShellArg(orchard) {
		t.Fatalf("tree %%d on nested row = %s, want %s", got, orchard)
	}
	nested := filepath.Join(orchard, "lantern.txt")
	state.SelectedPaths = map[string]bool{nested: true, filepath.Join(root, "ghost.txt"): false}
	if got, _ := ExpandCommand("%t", &state, nil); got != cmdmacro.QuoteShellArg(nested) {
		t.Fatalf("tree %%t = %s, want %s", got, nested)
	}
}
