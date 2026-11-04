package app

import (
	"testing"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// The delete dialog warns only when a marked file is in a potential duplicate group.
func TestDedupDeleteDialogWarnsAboutPotentialDuplicates(t *testing.T) {
	file := func(p string) comparepkg.DedupFile {
		return comparepkg.DedupFile{Rel: p[1:], Abs: pathloc.MustParse(p)}
	}
	a, _, _, _ := fileIndexApp(t)
	a.model.DedupSnapshot = comparepkg.DedupSnapshot{Groups: []comparepkg.DedupGroup{
		{Size: 9000, Files: []comparepkg.DedupFile{file("/lantern.bin"), file("/harbor.bin")}},
		{Size: 9000, Potential: true, Files: []comparepkg.DedupFile{file("/meadow.bin"), file("/willow.bin")}},
	}}
	a.model.DedupView.Marked = map[string]bool{"/lantern.bin": true}
	a.openDedupDeleteDialog()
	if a.model.FileDialog.Note != "" {
		t.Fatalf("note = %q with only a confirmed duplicate marked", a.model.FileDialog.Note)
	}
	a.model.DedupView.Marked["/willow.bin"] = true
	a.openDedupDeleteDialog()
	if a.model.FileDialog.Note != dedupPotentialDeleteNote {
		t.Fatalf("note = %q, want the potential duplicate warning", a.model.FileDialog.Note)
	}
}
