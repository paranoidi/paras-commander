package panel

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestRenameMarkAndDropOnLeave(t *testing.T) {
	t.Parallel()
	destDir := t.TempDir()
	dest := pathloc.MustParse(destDir)
	s := State{Path: dest}
	s.AddRenameMarks(dest, []string{"alpha.txt"})
	ent := localfs.Entry{Name: "alpha.txt", Path: destDir + "/alpha.txt", Type: localfs.EntryFile}
	if !s.IsRenameMarked(ent) {
		t.Fatal("expected rename mark on alpha.txt")
	}
	other := pathloc.MustParse(t.TempDir())
	if err := s.load(other, "", 10, noIndexCursorFallback, remoteLoadOpts{}); err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.IsRenameMarked(ent) {
		t.Fatal("rename marks should not apply after leaving dest directory")
	}
	if err := s.load(dest, "", 10, noIndexCursorFallback, remoteLoadOpts{}); err != nil {
		t.Fatalf("reload dest: %v", err)
	}
	if s.IsRenameMarked(ent) {
		t.Fatal("rename marks should be cleared when leaving dest and not restored on return")
	}
}
